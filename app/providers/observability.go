// Package providers wires process-wide cross-cutting providers that are not
// domain dependencies — currently OpenTelemetry (traces + metrics). Keeping this
// out of the container avoids coupling business wiring to the telemetry SDK.
package providers

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
)

// Observability holds the shutdown hooks for the configured telemetry pipelines.
type Observability struct {
	shutdownFns []func(context.Context) error
}

// SetupObservability installs the global trace/metric providers and propagator.
//
// The W3C propagator is always installed so trace context flows across HTTP/WS
// boundaries. When OTEL is disabled, the global no-op providers remain in place,
// so instrumentation code (otelhttp, spans) stays cheap and side-effect free.
// When enabled, stdout exporters are used by default — a concrete, dependency-
// free pipeline that can be swapped for OTLP later.
func SetupObservability(ctx context.Context, cfg config.OtelConfig) (*Observability, error) {
	obs := &Observability{}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if !cfg.Enabled {
		return obs, nil
	}

	res := resource.NewSchemaless(
		attribute.String("service.name", cfg.ServiceName),
	)

	traceExp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("otel trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	obs.shutdownFns = append(obs.shutdownFns, tp.Shutdown)

	metricExp, err := stdoutmetric.New()
	if err != nil {
		return nil, fmt.Errorf("otel metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)
	obs.shutdownFns = append(obs.shutdownFns, mp.Shutdown)

	return obs, nil
}

// Shutdown flushes and stops the telemetry pipelines in reverse order.
func (o *Observability) Shutdown(ctx context.Context) error {
	var firstErr error
	for i := len(o.shutdownFns) - 1; i >= 0; i-- {
		if err := o.shutdownFns[i](ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
