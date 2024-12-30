# OpenRouter API Wrapper 🏴‍☠️

A Go implementation of the OpenRouter API wrapper, providing a simple interface to interact with OpenRouter's API endpoints.

## Features
- CORS support
- Error handling with retries
- Streaming support for chat completions
- Simple proxy implementation

## Installation

1. Clone the repository:
```bash
git clone https://github.com/YOUR_USERNAME/openrouter-wrapper.git
cd openrouter-wrapper
```

2. Install dependencies:
```bash
go get github.com/gorilla/mux github.com/rs/cors
```

3. Run the server:
```bash
go run main.go
```

## API Endpoints

### POST /v1/chat/completions
Handle chat completions with optional streaming support.

**Parameters:**
- `stream` (query): Set to `true` for streaming response

**Example:**
```bash
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "Hello!"}]}'
```

### GET /v1/generation
Get generation statistics by ID.

**Parameters:**
- `id` (query): Generation ID

**Example:**
```bash
curl -X GET "http://localhost:3000/v1/generation?id=YOUR_GENERATION_ID" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### GET /v1/models
List available models.

**Example:**
```bash
curl -X GET http://localhost:3000/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

## Configuration

The server runs on port `3000` by default. To change the port, modify the `PORT` constant in `main.go`.

## License

MIT License - See [LICENSE](LICENSE) for details.

🏴‍☠️ Happy coding, matey! Arrr!
