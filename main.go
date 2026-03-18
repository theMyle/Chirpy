package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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
	content := fmt.Sprintf(`<html>
	<body>
	<h1>Welcome, Chirpy Admin</h1>
	<p>Chirpy has been visted  %d times!</p>
	</body>
	</html>
	`, cfg.fileServerHits.Load())

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(content))
}

// set metrics back to zero
func (cfg *apiConfig) handleMetricsReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileServerHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("Metrics reset to 0"))
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")

	type errorReturnVal struct {
		Error string `json:"error"`
	}

	respBody := errorReturnVal{
		Error: msg,
	}

	dat, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(code)
	w.Write([]byte(dat))
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) {
	dat, err := json.Marshal(payload)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
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
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		w.Write([]byte("OK"))
	})

	mux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
		type chirpRequest struct {
			Content string `json:"body"`
		}

		body := chirpRequest{}

		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			respondWithError(w, 500, "Something went wrong")
			return
		}

		// len validation
		if len(body.Content) > 140 {
			respondWithError(w, 400, "Chirp is too long")
			return
		}

		// profanity validation
		profanityList := []string{"kerfuffle", "sharbert", "fornax"}
		words := strings.Fields(strings.TrimSpace(body.Content))

		cleanedWords := make([]string, 0, len(words))

		for _, text := range words {
			isProfane := false

			for _, profanity := range profanityList {
				if strings.EqualFold(text, profanity) {
					isProfane = true
					break
				}
			}

			if isProfane {
				cleanedWords = append(cleanedWords, "****")
			} else {
				cleanedWords = append(cleanedWords, text)
			}
		}

		cleanedBody := strings.Join(cleanedWords, " ")
		respondWithJSON(w, 200, map[string]string{"cleaned_body": cleanedBody})
	})

	// metrics
	mux.HandleFunc("GET /admin/metrics", apiCfg.handleMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handleMetricsReset)

	srv := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	log.Printf("Running server at port %s (http://localhost%s)\n\n", srv.Addr, srv.Addr)

	fmt.Println("Routes:")
	fmt.Println("\tGET  /app:")
	fmt.Println("\tGET  /admin/metrics:")
	fmt.Println("\tPOST /admin/reset:")
	fmt.Println("\tGET  /api/healthz:")
	fmt.Println("\tPOST /api/validate_chirp:")

	srv.ListenAndServe()
}
