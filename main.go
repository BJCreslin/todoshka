package main

import (
	"log"
	"net/http"
	"os"

	"github.com/vladimirkreslin/todoshka/internal/server"
)

func main() {
	addr := os.Getenv("TODOSHKA_PORT")
	if addr == "" {
		addr = ":8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	log.Printf("todoshka starting on %s", addr)
	if err := http.ListenAndServe(addr, server.Wrap(mux)); err != nil {
		log.Fatal(err)
	}
}
