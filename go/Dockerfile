# Build stage
FROM golang:1.22-alpine AS builder

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

# Copy binary and .env from builder
COPY --from=builder /app/main .
COPY .env .

EXPOSE 5050

CMD ["./main"]
