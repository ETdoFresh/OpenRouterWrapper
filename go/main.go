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
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

// saveHistory saves request/response data to history directory
func saveHistory(data interface{}, prefix string) error {
	// Create history directory if it doesn't exist
	if err := os.MkdirAll("history", 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Generate filename with timestamp and prefix
	timestamp := time.Now().Format("20060102-150405.000")
	filename := fmt.Sprintf("%s-%s.json", timestamp, prefix)
	path := filepath.Join("history", filename)

	// Write file with atomic rename to prevent partial writes
	tempFile, err := os.CreateTemp("history", "temp-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode history data: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tempFile.Name(), path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

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

	// Save request history
	if err := saveHistory(map[string]interface{}{
		"method":  r.Method,
		"url":     r.URL.String(),
		"headers": r.Header,
		"body":    string(bodyBytes),
	}, "request"); err != nil {
		log.Printf("Failed to save request history: %v", err)
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
	var finalResponse struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index        int    `json:"index"`
			Message      struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	
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

		// Read and process streamed response
		decoder := json.NewDecoder(resp.Body)
		for decoder.More() {
			var chunk struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				Model   string `json:"model"`
				Choices []struct {
					Index        int    `json:"index"`
					Delta        struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}
			
			if err := decoder.Decode(&chunk); err != nil {
				log.Printf("🏴‍☠️ Error decoding chunk: %v", err)
				continue
			}

			// Initialize final response on first chunk
			if finalResponse.ID == "" {
				finalResponse.ID = chunk.ID
				finalResponse.Object = chunk.Object
				finalResponse.Created = chunk.Created
				finalResponse.Model = chunk.Model
				finalResponse.Choices = make([]struct {
					Index        int    `json:"index"`
					Message      struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				}, len(chunk.Choices))
			}

			// Accumulate content for each choice
			for i, choice := range chunk.Choices {
				finalResponse.Choices[i].Index = choice.Index
				finalResponse.Choices[i].Message.Content += choice.Delta.Content
				if choice.Delta.Role != "" {
					finalResponse.Choices[i].Message.Role = choice.Delta.Role
				}
				if choice.FinishReason != "" {
					finalResponse.Choices[i].FinishReason = choice.FinishReason
				}
			}
		}

		// Save complete response history
		if err := saveHistory(finalResponse, "response"); err != nil {
			log.Printf("Failed to save response history: %v", err)
		}

		// Return complete response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		if err := json.NewEncoder(w).Encode(finalResponse); err != nil {
			log.Printf("🏴‍☠️ Error encoding final response: %v", err)
		}
		return
	}

	http.Error(w, "Failed to get complete response after maximum retries", http.StatusInternalServerError)
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

	// Capture response
	var respBody bytes.Buffer
	tee := io.TeeReader(resp.Body, &respBody)

	// Save response history
	if err := saveHistory(map[string]interface{}{
		"status":     resp.Status,
		"headers":    resp.Header,
		"body":       respBody.String(),
		"target_url": targetURL,
	}, "response"); err != nil {
		log.Printf("Failed to save response history: %v", err)
	}

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, tee)
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
