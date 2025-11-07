# -------- Build Stage --------
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy Go modules and download deps
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN go build -o server ./cmd/server

# -------- Runtime Stage --------
FROM alpine:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server ./server

EXPOSE 8080
CMD ["./server"]
