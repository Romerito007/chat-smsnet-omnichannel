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

// buildHTTPHandler composes the API and/or WS routers onto a single mux based on
// the active roles.
func buildHTTPHandler(cfg config.Config, c *container.Container) http.Handler {
	root := chi.NewRouter()
	if cfg.RunsRole(config.RoleAPI) {
		root.Mount("/", httproutes.NewRouter(c))
	}
	if cfg.RunsRole(config.RoleWS) {
		root.Mount("/realtime", wsroutes.NewRouter(c))
	}
	return root
}
