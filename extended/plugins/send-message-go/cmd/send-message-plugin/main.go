package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/code-dot-org/kargo-send-message-step-plugin/internal/plugin"
)

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags|log.LUTC)
	server := plugin.NewServer(plugin.Options{
		Logger: logger,
	})

	addr := ":9765"
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logger.Printf("send-message step plugin listening on %s", addr)
	if err := httpServer.ListenAndServe(); err != nil {
		logger.Fatalf("send-message step plugin server exited: %v", err)
	}
}
