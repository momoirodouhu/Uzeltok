package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"uzeltok/internal/handler"
	"uzeltok/internal/store"
	"uzeltok/internal/web"
)

func main() {
	dataDir := "./data"
	if d := os.Getenv("UZELTOK_DATA_DIR"); d != "" {
		dataDir = d
	}

	ls := store.NewLinkStore(dataDir)

	vp, err := web.NewProvider()
	if err != nil {
		log.Fatalf("failed to initialize view provider: %v", err)
	}

	adminPass := os.Getenv("ADMIN_PASSWORD")

	const defaultMaxUploadBytes = 32 << 20 // 32MB
	maxUploadBytes := int64(defaultMaxUploadBytes)
	if s := os.Getenv("UPLOAD_MAX_BYTES"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 {
			log.Fatalf("invalid UPLOAD_MAX_BYTES: %q", s)
		}
		maxUploadBytes = n
	}

	h := handler.NewHandler(ls, vp, adminPass, maxUploadBytes)
	mux := http.NewServeMux()
	srv := h.Handler(mux)

	port := "8080"
	if p := os.Getenv("UZELTOK_PORT"); p != "" {
		if _, err := strconv.Atoi(p); err != nil {
			log.Fatalf("invalid UZELTOK_PORT: %q", p)
		}
		port = p
	}

	if adminPass != "" {
		fmt.Println("Admin access enabled (ADMIN_PASSWORD is set)")
	} else {
		fmt.Println("Admin access disabled (set ADMIN_PASSWORD to enable)")
	}

	fmt.Printf("Server starting on http://localhost:%s...\n", port)
	if err := http.ListenAndServe(":"+port, srv); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
