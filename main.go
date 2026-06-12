package main

import (
	"log"
	"net/http"
	"os"

	"github.com/vladimirkreslin/todoshka/internal/db"
	"github.com/vladimirkreslin/todoshka/internal/server"
)

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
	addr := os.Getenv("TODOSHKA_PORT")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("todoshka starting on %s (db=%s)", addr, dbPath)
	log.Fatal(http.ListenAndServe(addr, server.Wrap(mux)))
}
