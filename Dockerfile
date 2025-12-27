# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go build -o api ./cmd/api

# Run stage
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/api /app/api
COPY --from=builder /app/migrations /app/migrations
EXPOSE 8100
CMD ["/app/api"]
