package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"emba-api/config"
	"emba-api/database"
	"emba-api/handlers"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	cfg := config.Load()

	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/version", handlers.HandleVersion)
	mux.HandleFunc("GET /api/scan/profiles", handlers.HandleProfiles)
	mux.HandleFunc("POST /api/scan", handlers.HandleCreateScan)
	mux.HandleFunc("GET /api/scan", handlers.HandleListScans)
	mux.HandleFunc("GET /api/scan/{taskID}", handlers.HandleGetScan)
	mux.HandleFunc("GET /api/scan/{taskID}/logs", handlers.HandleGetLogs)
	mux.HandleFunc("GET /api/scan/{taskID}/report", handlers.HandleGetReport)
	mux.HandleFunc("GET /api/scan/{taskID}/sbom", handlers.HandleGetSBOM)
	mux.HandleFunc("DELETE /api/scan/{taskID}", handlers.HandleDeleteScan)
	mux.HandleFunc("GET /api/scan/{taskID}/events", handlers.HandleEvents)

	subFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to get static sub filesystem: %v", err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(subFS)))

	addr := cfg.Host + ":" + itoa(cfg.Port)
	log.Printf("EMBA API starting on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
