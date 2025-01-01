package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

const (
	OPENROUTER_API_URL = "https://openrouter.ai/api/v1"
	DEEPSEEK_API_URL   = "https://api.deepseek.com/chat/completions"
	PORT               = ":5050"
	MAX_RETRIES        = 3
)

var RETRY_DELAYS = []time.Duration{500 * time.Millisecond, 1000 * time.Millisecond, 3000 * time.Millisecond}

func main() {
	// Load .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

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

	// Start server with custom settings
	server := &http.Server{
		Addr:           PORT,
		Handler:        c.Handler(r),
		MaxHeaderBytes: 1 << 26, // 64MB
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
	}
	fmt.Printf("🏴‍☠️ Server be sailin' on port %s! Arrr!\n", PORT)
	log.Fatal(server.ListenAndServe())
}

func handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	stream := r.URL.Query().Get("stream") == "true"

	// Check if model is deepseek-chat
	var bodyBytes []byte
	var err error
	if r.Body != nil {
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Reset body for reuse
	}

	var requestBody map[string]interface{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}

	if model, ok := requestBody["model"].(string); ok && (model == "deepseek/deepseek-chat") {
		// Try DeepSeek API first
		// Update model to "deepseek-chat" for DeepSeek API
		requestBody["model"] = "deepseek-chat"
		updatedBody, _ := json.Marshal(requestBody)
		r.Body = io.NopCloser(bytes.NewBuffer(updatedBody)) // Reset body with updated model
		// Create new request with proper headers
		req, err := http.NewRequest("POST", DEEPSEEK_API_URL, r.Body)
		if err != nil {
			log.Printf("🏴‍☠️ Error creating DeepSeek request: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer " + os.Getenv("DEEPSEEK_API_KEY"))
		
		client := &http.Client{}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			return
		}
		log.Println("🏴‍☠️ DeepSeek API failed, falling back to OpenRouter")
	}

	if stream {
		handleStreamingChatCompletion(w, r)
		return
	}

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Reset body
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

		// Create a channel to handle stream timeout
		done := make(chan bool)
		timeout := time.NewTimer(15 * time.Second)
		
		go func() {
			// Stream the response
			_, err = io.Copy(w, resp.Body)
			done <- true
		}()

		select {
		case <-done:
			timeout.Stop()
			return
		case <-timeout.C:
			log.Printf("🏴‍☠️ Stream timeout! Retry attempt %d/%d\n", attempt+1, MAX_RETRIES)
			resp.Body.Close()
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
	// Limit request body size to 100MB
	body, err := io.ReadAll(io.LimitReader(r.Body, 100<<20)) // 100MB limit
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
	if attempt < len(RETRY_DELAYS) {
		return RETRY_DELAYS[attempt]
	}
	return RETRY_DELAYS[len(RETRY_DELAYS)-1]
}
