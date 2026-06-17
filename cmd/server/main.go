package main

import (
	"log"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/actionlab-ai/aisphere-auth/internal/server"
)

func main() {
	cfg := config.Load()

	srv := server.New(cfg)
	if err := srv.Run(); err != nil {
		log.Fatalf("aisphere-auth stopped: %v", err)
	}
}
