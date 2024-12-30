# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Final stage  
FROM alpine:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main .

EXPOSE 5050

CMD ["./main"]
