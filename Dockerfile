FROM golang:1.24-alpine

WORKDIR /app

# Copy Go modules (vendor included)
COPY go.mod go.sum ./
COPY vendor ./vendor

# Tell Go to use vendor
ENV GOFLAGS=-mod=vendor

COPY . .
RUN go build -o main ./cmd/server

EXPOSE 8080
CMD ["./main"]
