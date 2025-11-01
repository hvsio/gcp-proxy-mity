# Build stage
FROM golang:1.24.1-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /server cmd/server/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /server .

ENV PORT=8080
ENV GCS_BUCKET_NAME=aj-cloud
ENV GCP_PROJECT_ID=homey-bw58

EXPOSE 8080

CMD ["./server"]