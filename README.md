# OpenRouter API Wrapper 🏴‍☠️

Ahoy matey! This be a Node.js wrapper for the OpenRouter API, ready to set sail on yer local machine or Docker container. It provides a smooth interface for chat completions, generation stats, and model information.

## Features 🦜
- **Chat Completions**: Supports both streaming and non-streaming responses
- **Generation Stats**: Get details about specific generations
- **Model Information**: List available models
- **Robust Error Handling**: With retry logic for streaming
- **Docker Support**: Easy deployment with Docker

## Requirements 🏗️
- Node.js 18+
- Docker (optional)

## Installation 🛠️

### Local Setup
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

### Docker Deployment
1. Build and run the container:
   ```powershell
   ./dockerDeploy.ps1
   ```

2. The service will be available at:
   ```
   http://localhost:5050
   ```

## API Endpoints 🗺️

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

## Configuration ⚙️
- Port: 5050 (can be changed in server.js)
- OpenRouter API URL: https://openrouter.ai/api/v1

## Deployment 🚢
The included `dockerDeploy.ps1` script handles:
- Container cleanup
- Image building
- Container deployment
- Status verification
- Log display

## Troubleshooting 🚨
- Check container logs:
  ```bash
  docker logs openrouter-wrapper
  ```
- Verify container status:
  ```bash
  docker ps -a
  ```

## License 📜
ISC License - Free to use and modify, matey!

## Contributing 🤝
Pull requests are welcome! Let's make this ship even better!

## Support ☠️
If ye be having trouble, open an issue on GitHub or send a message in a bottle.

Happy sailing! 🏴‍☠️🦜
