FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates && update-ca-certificates

WORKDIR /workspace
COPY proto /workspace/proto
COPY services/search-service/go.mod services/search-service/go.sum /workspace/services/search-service/

WORKDIR /workspace/services/search-service
RUN go mod download

COPY services/search-service /workspace/services/search-service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
RUN addgroup -S app && adduser -S -G app -u 10001 app
WORKDIR /app
COPY --from=builder /workspace/services/search-service/main .
USER app
EXPOSE 50054
CMD ["./main"]
