package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"themyle/chirpy/internal/database"
	"themyle/chirpy/internal/handlers"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Server struct {
	Addr string
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalln("Error loading .env: ", err)
	}

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

	apiCfg := &handlers.APIConfig{}
	apiCfg.DBQueries = database.New(db)
	apiCfg.Platform = platform

	// file server
	mux.Handle("/app/",
		apiCfg.MiddlewareMetricsInc(
			http.StripPrefix("/app/", http.FileServer(http.Dir("."))),
		),
	)

	// health check
	mux.HandleFunc("GET /api/healthz", handlers.CheckHealth)
	mux.HandleFunc("POST /api/chirps", apiCfg.CreateChirp)
	mux.HandleFunc("POST /api/users", apiCfg.CreateUser)

	// metrics
	mux.HandleFunc("GET /admin/metrics", apiCfg.HandleMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.HandleMetricsReset)

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
	fmt.Println("\tPOST /api/chirps")

	err = srv.ListenAndServe()
	if err != nil {
		log.Println("Error starting server: ", err)
	}
}
