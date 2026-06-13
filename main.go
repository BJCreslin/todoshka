package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/vladimirkreslin/todoshka/internal/db"
	"github.com/vladimirkreslin/todoshka/internal/handlers"
	"github.com/vladimirkreslin/todoshka/internal/server"
)

//go:embed all:web
var webFS embed.FS

func main() {
	dbPath := os.Getenv("TODOSHKA_DB")
	if dbPath == "" {
		dbPath = "data/todoshka.db"
	}
	d, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer d.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if err := d.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"status":"ok"}`))
	})
	secret := os.Getenv("TODOSHKA_JWT_SECRET")
	if secret == "" { secret = "dev-secret-change-me-please-32bytes" }
	handlers.Mount(mux, d, secret)

	sub, err := fs.Sub(webFS, "web")
	if err != nil { log.Fatalf("embed web: %v", err) }
	mux.Handle("GET /", http.FileServer(http.FS(sub)))

	addr := os.Getenv("TODOSHKA_PORT")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("todoshka starting on %s (db=%s)", addr, dbPath)
	if err := http.ListenAndServe(addr, server.Wrap(mux)); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
