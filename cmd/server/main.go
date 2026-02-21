package main

import (
	"fmt"
	"log"
	"net/http"

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

	h := handler.NewHandler(ls, vp)
	h.RegisterRoutes(http.DefaultServeMux)

	fmt.Println("Server starting on http://localhost:8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
