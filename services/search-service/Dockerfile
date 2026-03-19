FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates && update-ca-certificates

WORKDIR /workspace
COPY proto /workspace/proto
COPY services/search-service/go.mod services/search-service/go.sum /workspace/services/search-service/

WORKDIR /workspace/services/search-service
RUN go mod download

COPY services/search-service /workspace/services/search-service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /workspace/services/search-service/main .
EXPOSE 50054
CMD ["./main"]
