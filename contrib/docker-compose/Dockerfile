# Multi-stage Dockerfile to build a small rtsper demo image
# Builds the binary using the Go toolchain and copies it into a slim runtime image.

FROM golang:1.24 AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /rtsper ./cmd/rtsper

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /rtsper /usr/local/bin/rtsper
EXPOSE 9191 9192 8080
ENTRYPOINT ["/usr/local/bin/rtsper"]
