FROM golang:1.24-alpin AS builder

RUN apk update && apk add --co-cache git ca-certificates && update-ca-certificates

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

