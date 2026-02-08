package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(100 * time.Millisecond)
		
		fmt.Fprintf(w, "Backend Server 2\n")
		fmt.Fprintf(w, "Request: %s %s\n", r.Method, r.URL.Path)
		fmt.Fprintf(w, "Time: %s\n", time.Now().Format("15:04:05"))
	})
	
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})
	
	log.Println("Test Backend 2 running on :9092")
	log.Fatal(http.ListenAndServe(":9092", nil))
}