package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Backend Server 1\n")
		fmt.Fprintf(w, "Request: %s %s\n", r.Method, r.URL.Path)
		fmt.Fprintf(w, "Time: %s\n", time.Now().Format("15:04:05"))
	})
	
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	log.Println("Test Backend 1 running on :9091")
	log.Fatal(http.ListenAndServe(":9091", nil))
}