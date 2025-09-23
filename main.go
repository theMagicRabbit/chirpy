package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}


func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK\n"))
}


func (cfg *apiConfig) handleAppHits(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	body := fmt.Sprintf("Hits: %d\n", cfg.fileserverHits.Load())
	w.Write([]byte(body))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) resetHitCounter(w http.ResponseWriter, _ *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Add("content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK\n"))
}

func main() {
	cfg := apiConfig{}
	mux := http.NewServeMux()
	fileServerHander := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/healthz", handleHealthz)
	mux.Handle("/app/", cfg.middlewareMetricsInc(fileServerHander))
	server := http.Server{
		Handler: mux,
		Addr: ":8080",
	}
	mux.HandleFunc("/metrics", cfg.handleAppHits)
	mux.HandleFunc("/reset", cfg.resetHitCounter)
	server.ListenAndServe()
}
