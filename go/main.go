package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

const (
	OPENROUTER_API_URL = "https://openrouter.ai/api/v1"
	PORT               = ":5050"
	MAX_RETRIES        = 5
	BASE_RETRY_DELAY   = 500 * time.Millisecond
	MAX_RETRY_DELAY    = 5 * time.Second
)

func main() {
	r := mux.NewRouter()
	
	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
	})

	// Routes
	r.HandleFunc("/v1/chat/completions", handleChatCompletion).Methods("POST")
	r.HandleFunc("/v1/generation", handleGeneration).Methods("GET")
	r.HandleFunc("/v1/models", handleModels).Methods("GET")

	// Start server
	handler := c.Handler(r)
	fmt.Printf("🏴‍☠️ Server be sailin' on port %s! Arrr!\n", PORT)
	log.Fatal(http.ListenAndServe(PORT, handler))
}

func handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	stream := r.URL.Query().Get("stream") == "true"

	if stream {
		handleStreamingChatCompletion(w, r)
		return
	}

	proxyRequest(w, r, OPENROUTER_API_URL+"/chat/completions")
}

func handleStreamingChatCompletion(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for attempt := 0; attempt < MAX_RETRIES; attempt++ {
		req, err := createProxyRequest(r, OPENROUTER_API_URL+"/chat/completions")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", r.Header.Get("Authorization"))
		req.Header.Set("HTTP-Referer", r.Header.Get("Referer"))
		req.Header.Set("X-Title", "OpenRouter API Wrapper")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("🏴‍☠️ Connection error! Retry attempt %d/%d\n", attempt+1, MAX_RETRIES)
			time.Sleep(calculateRetryDelay(attempt))
			continue
		}
		defer resp.Body.Close()

		// Stream the response
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			log.Printf("🏴‍☠️ Stream error! Retry attempt %d/%d\n", attempt+1, MAX_RETRIES)
			time.Sleep(calculateRetryDelay(attempt))
			continue
		}

		return
	}

	http.Error(w, "Failed to establish stream after maximum retries", http.StatusInternalServerError)
}

func handleGeneration(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, OPENROUTER_API_URL+"/generation?id="+r.URL.Query().Get("id"))
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, OPENROUTER_API_URL+"/models")
}

func proxyRequest(w http.ResponseWriter, r *http.Request, targetURL string) {
	req, err := createProxyRequest(r, targetURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		handleProxyError(w, err)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func createProxyRequest(r *http.Request, targetURL string) (*http.Request, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	copyHeaders(req.Header, r.Header)
	return req, nil
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func handleProxyError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "Internal server error"

	if errors.Is(err, context.DeadlineExceeded) {
		status = http.StatusGatewayTimeout
		message = "Gateway timeout"
	}

	http.Error(w, message, status)
}

func calculateRetryDelay(attempt int) time.Duration {
	delay := BASE_RETRY_DELAY * time.Duration(1<<uint(attempt))
	if delay > MAX_RETRY_DELAY {
		return MAX_RETRY_DELAY
	}
	return delay
}
