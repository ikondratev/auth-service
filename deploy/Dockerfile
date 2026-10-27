# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /app/bin/auth \
    ./cmd/auth   

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && adduser -D -g '' appuser

WORKDIR /app

COPY --from=builder /app/bin/auth /app/auth

USER appuser
EXPOSE 7777

CMD ["/app/http"]