package main

import (
	"bufio"
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
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

// saveHistory saves the request/response message pair
func saveHistory(data interface{}, timestamp string) error {
	// Create history directory if it doesn't exist
	if err := os.MkdirAll("history", 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Use provided timestamp for filename
	filename := fmt.Sprintf("%s.json", timestamp)
	path := filepath.Join("history", filename)

	// Write file with atomic rename to prevent partial writes
	tempFile, err := os.CreateTemp("history", "temp-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
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
	timestamp := time.Now().Format("20060102-150405.000")
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

	// Parse and re-encode request body to remove escaping
	var parsedBody interface{}
	if err := json.Unmarshal(bodyBytes, &parsedBody); err != nil {
		log.Printf("Failed to parse request body: %v", err)
		// Continue with original body if parsing fails
		parsedBody = string(bodyBytes)
	}

	stream = stream || (parsedBody != nil && parsedBody.(map[string]interface{})["stream"] == true)

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
		req.Header.Set("Authorization", "Bearer "+os.Getenv("DEEPSEEK_API_KEY"))

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
		handleStreamingChatCompletion(w, r, parsedBody, timestamp)
		return
	}

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Reset body
	proxyRequest(w, r, OPENROUTER_API_URL+"/chat/completions", timestamp, parsedBody, true)
}

func handleStreamingChatCompletion(w http.ResponseWriter, r *http.Request, parsedBody interface{}, timestamp string) {
	var finalResponse struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
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

		// Initialize the first choice with default values
		finalResponse.Choices = []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{{
			Index: 0,
			Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				Role:    "assistant",
				Content: "",
			},
		}}

		// Read and process streamed response
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip OpenRouter processing messages
			if strings.HasPrefix(line, ": ") {
				continue
			}

			// Only process data lines
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			// Extract JSON payload
			jsonData := strings.TrimPrefix(line, "data: ")
			if jsonData == "" {
				continue
			}

			var chunk struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				Model   string `json:"model"`
				Choices []struct {
					Index int `json:"index"`
					Delta struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(jsonData), &chunk); err != nil {
				log.Printf("🏴‍☠️ Error decoding chunk: %v", err)
				continue
			}

			// Initialize final response on first chunk
			if finalResponse.ID == "" {
				finalResponse.ID = chunk.ID
				finalResponse.Object = chunk.Object
				finalResponse.Created = chunk.Created
				finalResponse.Model = chunk.Model
			}

			// Accumulate content for each choice
			for i, choice := range chunk.Choices {
				if i >= len(finalResponse.Choices) {
					continue
				}
				finalResponse.Choices[i].Index = choice.Index
				finalResponse.Choices[i].Message.Content += choice.Delta.Content
				if choice.Delta.Role != "" {
					finalResponse.Choices[i].Message.Role = choice.Delta.Role
				}
				if choice.FinishReason != "" {
					finalResponse.Choices[i].FinishReason = choice.FinishReason
				}
			}

			// Write each chunk to the response
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			w.(http.Flusher).Flush()
		}

		if err := scanner.Err(); err != nil {
			log.Printf("🏴‍☠️ Error reading stream: %v", err)
			http.Error(w, "Error reading stream", http.StatusInternalServerError)
			return
		}

		// Extract messages from request
		var messages interface{}
		if reqMap, ok := parsedBody.(map[string]interface{}); ok {
			messages = reqMap["messages"]
		}

		// Save only the final request/response message data
		if err := saveHistory(map[string]interface{}{
			"request":  messages,
			"response": finalResponse.Choices[0].Message,
		}, timestamp); err != nil {
			log.Printf("Failed to save response history: %v", err)
		}

		return
	}

	http.Error(w, "Failed to get complete response after maximum retries", http.StatusInternalServerError)
}

func handleGeneration(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, OPENROUTER_API_URL+"/generation?id="+r.URL.Query().Get("id"), "", nil, false)
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, OPENROUTER_API_URL+"/models", "", nil, false)
}

func proxyRequest(w http.ResponseWriter, r *http.Request, targetURL string, timestamp string, parsedBody interface{}, saveToHistory bool) {
	// Read the request body first
	var requestBody interface{}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Parse the request body
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
			log.Printf("Failed to parse request body: %v", err)
		}
	}

	// Reset the body for the proxy request
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Create and send the proxy request
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

	// Read the response body
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}

	// Only try to parse JSON for successful responses
	var responseBody interface{}
	if resp.StatusCode == http.StatusOK && strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(respBytes, &responseBody); err != nil {
			log.Printf("Failed to parse response body: %v", err)
			responseBody = string(respBytes)
		}
	} else {
		responseBody = string(respBytes)
	}

	if saveToHistory {
		// Extract messages from request
		var messages interface{}
		if parsedBody != nil {
			if reqMap, ok := parsedBody.(map[string]interface{}); ok {
				messages = reqMap["messages"]
			}
		}

		// Extract response message
		var responseMessage interface{}
		if respMap, ok := responseBody.(map[string]interface{}); ok {
			if choices, ok := respMap["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if message, ok := choice["message"].(map[string]interface{}); ok {
						responseMessage = message
					}
				}
			}
		}

		// Save request/response data
		if err := saveHistory(map[string]interface{}{
			"request":  messages,
			"response": responseMessage,
		}, timestamp); err != nil {
			log.Printf("Failed to save response history: %v", err)
		}
	}

	// Reset response body for writing back to client
	resp.Body = io.NopCloser(bytes.NewBuffer(respBytes))

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
