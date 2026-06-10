// Package app is the top-level wiring entrypoint invoked by main. It loads
// configuration and hands off to the start routines for the active role, or runs
// a one-shot subcommand (e.g. `seed`).
package app

import (
	"context"
	"fmt"
	"os"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
	"github.com/romerito007/chat-smsnet-omnichannel/app/start_routines"
)

// Run loads configuration and either runs a one-shot subcommand or starts the
// long-running process for the active role. It blocks until shutdown.
//
// Usage:
//
//	chat-backend          # start per RUN_ROLE (all|api|ws|worker|scheduler)
//	chat-backend seed     # run idempotent seed (tenant + owner + roles) and exit
func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "seed":
			return start_routines.Seed(ctx, cfg)
		default:
			return fmt.Errorf("unknown command %q", os.Args[1])
		}
	}

	return start_routines.Start(ctx, cfg)
}
