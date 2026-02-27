# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /edgeproxy ./cmd/edgeproxy

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /edgeproxy /usr/local/bin/edgeproxy

EXPOSE 8080 8081

ENTRYPOINT ["edgeproxy"]
