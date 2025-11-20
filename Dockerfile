# -------- Build Stage --------
FROM golang:1.23 AS builder

WORKDIR /app

# Copy Go modules and download deps
COPY go.mod go.sum ./
ARG GOFLAGS
ARG GOPROXY
ENV GOFLAGS=${GOFLAGS}
ENV GOPROXY=${GOPROXY}
RUN go env && go mod download

# Copy source code
COPY . .

# Build static binary (no CGO)
ENV CGO_ENABLED=0
RUN go version && go list -m all && go build -v -trimpath -ldflags "-s -w" -o server ./cmd/api

# -------- Runtime Stage --------
FROM gcr.io/distroless/base-debian12

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server /app/server

EXPOSE 8080
STOPSIGNAL SIGTERM
ENTRYPOINT ["/app/server"]
