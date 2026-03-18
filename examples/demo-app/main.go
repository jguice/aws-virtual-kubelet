package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	name := os.Getenv("APP_NAME")
	if name == "" {
		name = "vk-demo"
	}

	startTime := time.Now()

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		uptime := time.Since(startTime).Round(time.Second)
		hostname, _ := os.Hostname()
		fmt.Fprintf(w, "Hello from %s!\nHostname: %s\nUptime: %s\nPID: %d\n",
			name, hostname, uptime, os.Getpid())
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ready")
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Handle graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-done
		log.Printf("Received signal %v, shutting down...", sig)
		server.Close()
	}()

	log.Printf("Starting %s on port %s (pid %d)", name, port, os.Getpid())
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Printf("Server stopped cleanly")
}
