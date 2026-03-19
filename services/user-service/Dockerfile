FROM golang:1.24-alpine AS builder

# Install git and ca-certificates
RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates

# Set working directory
WORKDIR /workspace

# Copy shared proto module and service dependencies
COPY proto /workspace/proto
COPY services/user-service/go.mod services/user-service/go.sum /workspace/services/user-service/

# Download dependencies
WORKDIR /workspace/services/user-service
RUN go mod download

# Copy source code
COPY services/user-service /workspace/services/user-service

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder stage
COPY --from=builder /workspace/services/user-service/main .

# Expose port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

# Run the application
CMD ["./main"]
