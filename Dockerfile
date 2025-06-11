# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -o app .

# Final image
FROM alpine:latest

WORKDIR /app

# Copy static assets and templates
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates

# Copy the built binary
COPY --from=builder /app/app .

# Expose the port your app listens on (default 8081)
EXPOSE 8081

# Set environment variables (override at runtime as needed)
ENV PORT=8081

# Run the app
CMD ["./app"]