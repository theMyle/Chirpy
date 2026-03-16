package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type Server struct {
	Addr string
}

type apiConfig struct {
	fileServerHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// show metrics (hit count)
func (cfg *apiConfig) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("Hits: " + fmt.Sprint(cfg.fileServerHits.Load())))
}

// set metrics back to zero
func (cfg *apiConfig) handleMetricsReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileServerHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("Metrics reset to 0"))
}

func main() {
	mux := http.NewServeMux()
	apiCfg := &apiConfig{}

	// file server
	mux.Handle("/app/",
		apiCfg.middlewareMetricsInc(
			http.StripPrefix("/app/", http.FileServer(http.Dir("."))),
		),
	)

	// health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		w.Write([]byte("OK"))
	})

	// metrics
	mux.HandleFunc("GET /metrics", apiCfg.handleMetrics)
	mux.HandleFunc("POST /reset", apiCfg.handleMetricsReset)

	srv := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	log.Printf("Running server at port %s (http://localhost%s)\n\n", srv.Addr, srv.Addr)

	fmt.Println("Routes:")
	fmt.Println("\t/app:")
	fmt.Println("\t/healthz:")

	srv.ListenAndServe()
}
