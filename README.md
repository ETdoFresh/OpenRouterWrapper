# OpenRouter API Wrapper 🏴‍☠️⚡️

Ahoy matey! This be a dual-language implementation of the OpenRouter API wrapper, available in both Go and Node.js. Choose yer weapon of choice to interact with OpenRouter's API endpoints!

## Features 🏴‍☠️
- **Chat Completions**: Supports both streaming and non-streaming responses
- **Generation Stats**: Get details about specific generations
- **Model Information**: List available models
- **Robust Error Handling**: With retry logic for streaming
- **Docker Support**: Easy deployment with Docker
- **CORS Support**: For cross-origin requests
- **Simple Proxy Implementation**: Easy to integrate

## Choose Yer Language 🏴‍☠️

### Go Implementation
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

### Node.js Implementation
1. Clone the repository:
```bash
git clone https://github.com/your-repo/openrouter-wrapper.git
cd openrouter-wrapper
```

2. Install dependencies:
```bash
npm install
```

3. Start the server:
```bash
npm start
```

For development with auto-reload:
```bash
npm run dev
```

## API Endpoints 🏴‍☠️

### Chat Completions
`POST /v1/chat/completions`
- Supports streaming with `stream: true`
- Requires Authorization header with OpenRouter API key

### Generation Stats
`GET /v1/generation?id=<generation_id>`
- Get details about a specific generation

### Models
`GET /v1/models`
- List available models

## Docker Deployment 🏴‍☠️

Both implementations include Docker support:

### Go
```powershell
./go/DockerDeploy.ps1
```

### Node.js
```powershell
./nodejs/DockerDeploy.ps1
```

## Configuration 🏴‍☠️

### Go
- Port: 3000 (modify the `PORT` constant in `main.go`)

### Node.js
- Port: 5050 (can be changed in server.js)
- OpenRouter API URL: https://openrouter.ai/api/v1

## Troubleshooting 🏴‍☠️

### Go
- Check server logs for errors

### Node.js
- Check container logs:
```bash
docker logs openrouter-wrapper
```
- Verify container status:
```bash
docker ps -a
```

## License 🏴‍☠️
MIT License - See [LICENSE](LICENSE) for details.

## Contributing 🏴‍☠️
Pull requests are welcome! Let's make this ship even better!

## Support 🏴‍☠️
If ye be having trouble, open an issue on GitHub or send a message in a bottle.

Happy sailing! 🏴‍☠️⚡️
