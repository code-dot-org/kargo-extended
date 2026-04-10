package main

import (
	"log"
	"net/http"
	"os"

	"github.com/code-dot-org/kargo-send-message-step-plugin/internal/plugin"
)

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags|log.LUTC)
	server := plugin.NewServer(plugin.Options{
		Logger: logger,
	})

	addr := ":9765"
	logger.Printf("send-message step plugin listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		logger.Fatalf("send-message step plugin server exited: %v", err)
	}
}
