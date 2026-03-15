package main

import "net/http"

type Server struct {
	Addr string
}

func main()  {
	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.Dir(".")))

	server := &http.Server{
		Handler: mux,
		Addr: ":8080",
	}

	server.ListenAndServe()
}