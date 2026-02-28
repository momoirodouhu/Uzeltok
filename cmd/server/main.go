package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"uzeltok/internal/handler"
	"uzeltok/internal/store"
	"uzeltok/internal/web"
)

func main() {
	ls := store.NewLinkStore("./data")

	vp, err := web.NewProvider()
	if err != nil {
		log.Fatalf("failed to initialize view provider: %v", err)
	}

	adminPass := os.Getenv("ADMIN_PASSWORD")

	h := handler.NewHandler(ls, vp, adminPass)
	mux := http.NewServeMux()
	srv := h.Handler(mux)

	if adminPass != "" {
		fmt.Println("Admin access enabled (ADMIN_PASSWORD is set)")
	} else {
		fmt.Println("Admin access disabled (set ADMIN_PASSWORD to enable)")
	}

	fmt.Println("Server starting on http://localhost:8080...")
	if err := http.ListenAndServe(":8080", srv); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
