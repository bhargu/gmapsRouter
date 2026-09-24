# Multi-stage build: compile with Go, run with a minimal base

# 1) Build stage
FROM golang:1.25 AS builder
WORKDIR /src

# Leverage caching for dependencies
COPY go.mod ./
# If you have go.sum, copy it too to improve caching
# COPY go.sum ./
RUN go mod download

# Copy the rest of the source
COPY main.go google.go ./

# Build a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/routerApp .

# 2) Runtime stage (distroless with non-root user and CA certs)
FROM gcr.io/distroless/static:nonroot
WORKDIR /app

COPY --from=builder /out/routerApp /app/routerApp

EXPOSE 8080

ENTRYPOINT ["/app/routerApp"]