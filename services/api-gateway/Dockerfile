# Dockerfile
FROM golang:1.24-alpine AS builder

# Install git and ca-certificates
RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates

# Set working directory
WORKDIR /workspace

# Copy shared proto module and service dependencies
COPY proto /workspace/proto
COPY services/api-gateway/go.mod services/api-gateway/go.sum /workspace/services/api-gateway/

# Download dependencies
WORKDIR /workspace/services/api-gateway
RUN go mod download

# Copy source code
COPY services/api-gateway /workspace/services/api-gateway

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates and wget for health checks
RUN apk --no-cache add ca-certificates wget

WORKDIR /root/

# Copy binary from builder stage
COPY --from=builder /workspace/services/api-gateway/main .

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --output-document=/dev/null http://localhost:8080/health || exit 1

# Run the application
CMD ["./main"]
