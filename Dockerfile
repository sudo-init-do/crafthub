# -------- Build Stage --------
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy Go modules (vendor included)
COPY go.mod go.sum ./
COPY vendor ./vendor
ENV GOFLAGS=-mod=vendor

# Copy source code
COPY . .

# Build binary
RUN go build -o main ./cmd/server

# -------- Runtime Stage --------
FROM alpine:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main .

EXPOSE 8080
CMD ["./main"]
