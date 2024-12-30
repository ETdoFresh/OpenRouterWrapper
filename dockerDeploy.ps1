# Docker deployment script for OpenRouter Wrapper
$ErrorActionPreference = "Stop"
$containerName = "openrouter-wrapper"
$imageName = "openrouter-wrapper"
$port = 5050

Write-Host "[*] Ahoy! Starting deployment of $containerName..." -ForegroundColor Cyan

try {
    # Check if container exists and remove it
    Write-Host "[*] Checking for existing containers..." -ForegroundColor Yellow
    $existingContainer = docker ps -a --filter "name=$containerName" --format "{{.Names}}"
    if ($existingContainer) {
        Write-Host "[!] Found existing container. Walking the plank with ye!" -ForegroundColor Yellow
        docker stop $containerName
        docker rm $containerName
    }

    # Build the new image
    Write-Host "[*] Building fresh Docker image..." -ForegroundColor Yellow
    docker build -t $imageName .

    # Run the new container
    Write-Host "[*] Launching new container..." -ForegroundColor Yellow
    docker run -d `
        --name $containerName `
        -p "${port}:${port}" `
        --restart unless-stopped `
        --env-file .env `
        $imageName

    # Check if container is running
    $runningContainer = docker ps --filter "name=$containerName" --format "{{.Names}}"
    if ($runningContainer) {
        Write-Host "[+] Yarr! The ship has set sail! Container is running successfully!" -ForegroundColor Green
        Write-Host "[+] Service available at http://localhost:$port" -ForegroundColor Cyan
        
        # Display container logs
        Write-Host "`n[*] Here be the container logs:" -ForegroundColor Yellow
        docker logs $containerName
    } else {
        throw "Container failed to start! Check the logs with: docker logs $containerName"
    }

} catch {
    Write-Host "[-] Blimey! Something went wrong: $_" -ForegroundColor Red
    exit 1
}
