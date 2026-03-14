package main

import (
	"fmt"
	"log"
	"net/http"

	"uzeltok/internal/config"
	"uzeltok/internal/handler"
	"uzeltok/internal/store"
	"uzeltok/internal/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	ls := store.NewLinkStore(cfg.DataDir)

	vp, err := web.NewProvider()
	if err != nil {
		log.Fatalf("failed to initialize view provider: %v", err)
	}

	h, err := handler.NewHandler(ls, vp, cfg)
	if err != nil {
		log.Fatalf("failed to initialize handler: %v", err)
	}
	mux := http.NewServeMux()
	srv := h.Handler(mux)

	if cfg.AdminPassword != "" {
		fmt.Println("Admin access enabled (ADMIN_PASSWORD is set)")
	} else {
		fmt.Println("Admin access disabled (set ADMIN_PASSWORD to enable)")
	}

	fmt.Printf("Server starting on http://localhost:%s...\n", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, srv); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
