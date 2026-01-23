package main

import (
	"fmt"
	"net/http"

	"uzeltok/internal/handler"
	"uzeltok/internal/store"
)

func main() {
	// データディレクトリを基準にストアを初期化（必要に応じて変更してください）
	ls := store.NewLinkStore("./data")
	h := handler.NewHandler(ls)
	h.RegisterRoutes(http.DefaultServeMux)

	fmt.Println("Server starting on http://localhost:8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
