// Package app is the top-level wiring entrypoint invoked by main. It loads
// configuration and hands off to the start routines for the active role.
package app

import (
	"context"
	"fmt"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
	"github.com/romerito007/chat-smsnet-omnichannel/app/start_routines"
)

// Run loads configuration and starts the process. It blocks until shutdown.
func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return start_routines.Start(ctx, cfg)
}
