package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"themyle/chirpy/internal/database"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Server struct {
	Addr string
}

type apiConfig struct {
	fileServerHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
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
	if cfg.platform == "dev" {
		err := cfg.dbQueries.DeleteAllUser(r.Context())
		if err != nil {
			w.WriteHeader(500)
			return
		}

		cfg.fileServerHits.Store(0)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("Metrics reset to 0 & Cleared DB\n"))
		return
	}

	w.WriteHeader(403)
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

func checkHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	w.Write([]byte("OK"))
}

func validateChirp(w http.ResponseWriter, r *http.Request) {
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
}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	// parse request body
	type createUserRequest struct {
		Email string `json:"email"`
	}

	reqBody := createUserRequest{}
	json.NewDecoder(r.Body).Decode(&reqBody)

	// create new user in DB
	res, err := cfg.dbQueries.CreateUser(r.Context(), reqBody.Email)
	if err != nil {
		log.Printf("ERROR - %s\n", err)
		respondWithError(w, 400, "Invalid request payload")
		return
	}

	// assemble response
	userResponse := User{
		ID:        res.ID,
		CreatedAt: res.CreatedAt,
		UpdatedAt: res.UpdatedAt,
		Email:     res.Email,
	}

	respondWithJSON(w, 201, userResponse)
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")

	fmt.Println("[ DEBUG ]")
	fmt.Printf("DB_URL: %s\n", dbURL)
	fmt.Printf("Platform: %s\n\n", platform)

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error connecting to DB: %s\n", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatalf("Error connecting to DB: %s\n", err)
	}

	mux := http.NewServeMux()

	apiCfg := &apiConfig{}
	apiCfg.dbQueries = database.New(db)
	apiCfg.platform = platform

	// file server
	mux.Handle("/app/",
		apiCfg.middlewareMetricsInc(
			http.StripPrefix("/app/", http.FileServer(http.Dir("."))),
		),
	)

	// health check
	mux.HandleFunc("GET /api/healthz", checkHealth)
	mux.HandleFunc("POST /api/validate_chirp", validateChirp)
	mux.HandleFunc("POST /api/users", apiCfg.createUser)

	// metrics
	mux.HandleFunc("GET /admin/metrics", apiCfg.handleMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handleMetricsReset)

	srv := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	log.Printf("Running server at port %s (http://localhost%s)\n\n", srv.Addr, srv.Addr)

	fmt.Println("Routes:")
	fmt.Println("\tGET  /app")
	fmt.Println("\tGET  /admin/metrics")
	fmt.Println("\tGET  /api/healthz")

	fmt.Println("\tPOST /admin/reset")
	fmt.Println("\tPOST /api/validate_chirp")
	fmt.Println("\tPOST /api/users")

	srv.ListenAndServe()
}
