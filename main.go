// Command chat-backend is the single binary for the omnichannel chat backend.
// The active role is selected at runtime via the RUN_ROLE environment variable
// (all | api | ws | worker | scheduler).
package main

import (
	"context"
	"log"

	"github.com/romerito007/chat-smsnet-omnichannel/app"
)

func main() {
	if err := app.Run(context.Background()); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}
