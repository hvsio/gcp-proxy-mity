# Build stage
FROM golang:1.24.1-alpine AS builder

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./

RUN go mod download

COPY . .

EXPOSE 8080

ENV GCS_BUCKET_NAME=aj-cloud
ENV PROJECT_ID=homey-bw58

ENTRYPOINT ["go", "run", "cmd/server/main.go"]