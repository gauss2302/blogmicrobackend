FROM golang:1.24-alpine AS builder

# Install git and ca-certificates (needed for HTTPS requests)
RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates

# Set working directory
WORKDIR /workspace

# Copy shared proto module and service dependencies
COPY proto /workspace/proto
COPY services/auth-service/go.mod services/auth-service/go.sum /workspace/services/auth-service/

# Download dependencies
WORKDIR /workspace/services/auth-service
RUN go mod download

# Copy source code
COPY services/auth-service /workspace/services/auth-service

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder stage
COPY --from=builder /workspace/services/auth-service/main .

# Expose port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

# Run the application
CMD ["./main"]
