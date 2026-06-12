// Package start_routines is the role dispatcher: it reads RUN_ROLE and boots the
// routines (HTTP API, WebSocket, Asynq worker, Asynq scheduler) belonging to the
// active role, sharing a single container. A single binary can run any subset.
package start_routines

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/providers"
	httproutes "github.com/romerito007/chat-smsnet-omnichannel/app/routes/http"
	wsroutes "github.com/romerito007/chat-smsnet-omnichannel/app/routes/websocket"
	"github.com/romerito007/chat-smsnet-omnichannel/app/server"
)

// Start builds the container, runs first-boot bootstrap (indexes + seeds) and
// launches the routines for the active role, blocking until a signal or fatal
// error.
func Start(ctx context.Context, cfg config.Config) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Telemetry first, so every subsequent component is instrumented.
	obs, err := providers.SetupObservability(ctx, cfg.Otel)
	if err != nil {
		return fmt.Errorf("setup observability: %w", err)
	}
	defer func() { _ = obs.Shutdown(context.Background()) }()

	c, err := container.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build container: %w", err)
	}
	defer c.Close(context.Background())

	// First-boot schema work runs for any role that owns data setup. Indexes and
	// seeds are idempotent, so running them on api/all is safe and cheap.
	if cfg.RunsRole(config.RoleAPI) || cfg.RunRole == config.RoleAll {
		if err := bootstrapMongo(ctx, c); err != nil {
			return err
		}
		if err := bootstrapIndexes(ctx, c); err != nil {
			return err
		}
		if err := bootstrapSeeds(ctx, c); err != nil {
			return err
		}
		// Demo data (gated by SEED_DEMO_DATA, dev only). Runs after the real seed
		// and reconcile, only on api/all (never on every worker). Non-fatal: a
		// demo-data hiccup must never block API startup.
		if err := SeedDemoData(ctx, c); err != nil {
			c.Logger.Warn("demo seed failed (continuing); set SEED_DEMO_RESET=true to retry clean", "error", err)
		}
	}

	g, gctx := errgroup.WithContext(ctx)

	// API and WS share one HTTP listener; mount whichever role is active.
	if cfg.RunsRole(config.RoleAPI) || cfg.RunsRole(config.RoleWS) {
		handler := buildHTTPHandler(cfg, c)
		srv := server.New(cfg.HTTP, handler, c.Logger)
		g.Go(func() error { return srv.Run(gctx) })
	}

	// WS also needs the cross-node pub/sub loop running.
	if cfg.RunsRole(config.RoleWS) {
		g.Go(func() error {
			err := c.Realtime.Run(gctx)
			if err == context.Canceled {
				return nil
			}
			return err
		})
	}

	if cfg.RunsRole(config.RoleWorker) {
		g.Go(func() error { return runWorker(gctx, c) })
	}

	if cfg.RunsRole(config.RoleScheduler) {
		g.Go(func() error { return runScheduler(gctx, c) })
	}

	c.Logger.Info("started", "role", cfg.RunRole)
	if err := g.Wait(); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

// Seed runs the first-boot schema work (mongo check, indexes, seed) once and
// exits. It backs the `chat-backend seed` command (`make seed`) and is fully
// idempotent: re-running creates nothing new.
func Seed(ctx context.Context, cfg config.Config) error {
	c, err := container.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build container: %w", err)
	}
	defer c.Close(context.Background())

	if err := bootstrapMongo(ctx, c); err != nil {
		return err
	}
	if err := bootstrapIndexes(ctx, c); err != nil {
		return err
	}
	if err := bootstrapSeeds(ctx, c); err != nil {
		return err
	}
	c.Logger.Info("seed completed")
	return nil
}

// buildHTTPHandler composes the API and/or WS routers onto a single mux based on
// the active roles.
func buildHTTPHandler(cfg config.Config, c *container.Container) http.Handler {
	var api, wsHandler http.Handler
	if cfg.RunsRole(config.RoleAPI) {
		api = httproutes.NewRouter(c)
	}
	if cfg.RunsRole(config.RoleWS) {
		wsHandler = wsroutes.Handler(c)
	}
	return composeRouter(api, wsHandler)
}

// composeRouter mounts the API as the catch-all and exposes the WS handler at
// both /realtime/ws (canonical) and /ws (the alias browsers commonly connect to),
// so the WebSocket upgrade is never swallowed by the API 404. Either handler may
// be nil when its role is inactive. The static WS routes take precedence over the
// API catch-all.
func composeRouter(api, wsHandler http.Handler) http.Handler {
	root := chi.NewRouter()
	if wsHandler != nil {
		root.Handle("/realtime/ws", wsHandler)
		root.Handle("/ws", wsHandler)
	}
	if api != nil {
		root.Mount("/", api)
	}
	return root
}
