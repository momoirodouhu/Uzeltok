package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

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
		n, err := parseByteSize(s)
		if err != nil {
			log.Fatalf("invalid UPLOAD_MAX_BYTES: %q (%v)", s, err)
		}
		maxUploadBytes = n
	}

	h, err := handler.NewHandler(ls, vp, adminPass, maxUploadBytes)
	if err != nil {
		log.Fatalf("failed to initialize handler: %v", err)
	}
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

func parseByteSize(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("value is empty")
	}

	i := 0
	dotCount := 0
	for i < len(s) {
		c := s[i]
		if c >= '0' && c <= '9' {
			i++
			continue
		}
		if c == '.' {
			dotCount++
			if dotCount > 1 {
				return 0, fmt.Errorf("invalid number")
			}
			i++
			continue
		}
		break
	}

	if i == 0 {
		return 0, fmt.Errorf("missing numeric value")
	}

	numPart := s[:i]
	suffix := strings.ToUpper(strings.TrimSpace(s[i:]))

	n, err := strconv.ParseFloat(numPart, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid numeric value")
	}

	mul := float64(1)
	switch suffix {
	case "", "B":
		mul = 1
	case "K", "KB":
		mul = 1000
	case "M", "MB":
		mul = 1000 * 1000
	case "G", "GB":
		mul = 1000 * 1000 * 1000
	case "T", "TB":
		mul = 1000 * 1000 * 1000 * 1000
	case "P", "PB":
		mul = 1000 * 1000 * 1000 * 1000 * 1000
	case "KI", "KIB":
		mul = 1 << 10
	case "MI", "MIB":
		mul = 1 << 20
	case "GI", "GIB":
		mul = 1 << 30
	case "TI", "TIB":
		mul = 1 << 40
	case "PI", "PIB":
		mul = 1 << 50
	default:
		return 0, fmt.Errorf("unsupported size suffix (use bytes or units like MiB/GB)")
	}

	bytes := n * mul
	if bytes < 1 || bytes > float64(math.MaxInt64) {
		return 0, fmt.Errorf("size is out of range")
	}

	return int64(math.Round(bytes)), nil
}
