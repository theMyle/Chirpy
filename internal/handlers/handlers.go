// Package handlers
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"themyle/chirpy/internal/database"

	"github.com/google/uuid"
)

type APIConfig struct {
	FileServerHits atomic.Int32
	DBQueries      *database.Queries
	Platform       string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func (cfg *APIConfig) MiddlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.FileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// HandleMetrics - show metrics (hit count)
func (cfg *APIConfig) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	content := fmt.Sprintf(`<html>
	<body>
	<h1>Welcome, Chirpy Admin</h1>
	<p>Chirpy has been visted  %d times!</p>
	</body>
	</html>
	`, cfg.FileServerHits.Load())

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(content))
}

// HandleMetricsReset - metrics back to zero
func (cfg *APIConfig) HandleMetricsReset(w http.ResponseWriter, r *http.Request) {
	if cfg.Platform == "dev" {
		err := cfg.DBQueries.DeleteAllUser(r.Context())
		if err != nil {
			w.WriteHeader(500)
			return
		}

		cfg.FileServerHits.Store(0)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Metrics reset to 0 & Cleared DB\n"))
		return
	}

	w.WriteHeader(403)
}

func RespondWithError(w http.ResponseWriter, code int, msg string) {
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
	_, _ = w.Write([]byte(dat))
}

func RespondWithJSON(w http.ResponseWriter, code int, payload any) {
	dat, err := json.Marshal(payload)
	if err != nil {
		RespondWithError(w, 500, "Something went wrong")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(dat)
}

func CheckHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte("OK"))
}

func (cfg *APIConfig) CreateUser(w http.ResponseWriter, r *http.Request) {
	// parse request body
	type createUserRequest struct {
		Email string `json:"email"`
	}

	reqBody := createUserRequest{}

	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Printf("ERROR - %s\n", err)
		RespondWithError(w, 400, "Invalid request payload")
		return
	}

	// create new user in DB
	res, err := cfg.DBQueries.CreateUser(r.Context(), reqBody.Email)
	if err != nil {
		log.Printf("ERROR - %s\n", err)
		RespondWithError(w, 409, "Email already taken")
		return
	}

	// assemble response
	userResponse := User{
		ID:        res.ID,
		CreatedAt: res.CreatedAt,
		UpdatedAt: res.UpdatedAt,
		Email:     res.Email,
	}

	RespondWithJSON(w, 201, userResponse)
}

func (cfg *APIConfig) CreateChirp(w http.ResponseWriter, r *http.Request) {
	type CreateChirpRequest struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}

	type CreateChirpResponse struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}

	req := CreateChirpRequest{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("Error decoding JSON: %s\n", err)
		RespondWithError(w, 400, "Invalid Request Payload")
		return
	}

	// validate chirp
	if len(req.Body) > 140 {
		RespondWithError(w, 400, "Chirp is too long")
		return
	}

	profanityList := []string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Fields(strings.TrimSpace(req.Body))
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
	req.Body = cleanedBody

	// insert to DB
	params := database.CreateChirpParams{
		UserID: req.UserID,
		Body:   req.Body,
	}

	chirp, err := cfg.DBQueries.CreateChirp(r.Context(), params)
	if err != nil {
		log.Printf("Error writing Chirp to DB: %s\n", err)
		RespondWithError(w, 500, "Server Error\n")
		return
	}

	response := CreateChirpResponse{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}

	RespondWithJSON(w, 201, response)
}
