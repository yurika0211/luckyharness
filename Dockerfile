# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o luckyharness ./cmd/lh

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/luckyharness /usr/local/bin/luckyharness

# Create config directory
RUN mkdir -p /etc/luckyharness /var/lib/luckyharness

VOLUME ["/etc/luckyharness", "/var/lib/luckyharness"]

ENTRYPOINT ["luckyharness"]
CMD ["--help"]