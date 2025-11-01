# Build stage
FROM golang:1.24.1-alpine AS builder

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./

RUN go mod download

COPY . .

EXPOSE 8080

RUN echo $GCS_BUCKET_NAME
RUN echo $PROJECT_ID
RUN echo $STORAGE_GOOGLE_APPLICATION_CREDENTIALS

ENTRYPOINT ["go", "run", "cmd/server/main.go"]