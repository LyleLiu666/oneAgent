package main

import (
	"log"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/server"
)

func main() {
	cfg, err := config.Load(config.LoadOptions{})
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	rt, err := runtime.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to init runtime: %v", err)
	}
	defer func() {
		_ = rt.Close()
	}()

	if err := server.Serve(rt); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
