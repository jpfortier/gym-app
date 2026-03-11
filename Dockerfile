FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    CGO_ENABLED=0 go build -ldflags "-X github.com/jpfortier/gym-app/internal/env.buildDate=$$BUILD_DATE" -o /api ./cmd/api

RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

FROM alpine:3.19

RUN apk add --no-cache ca-certificates

COPY --from=builder /api /api
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY --from=builder /app/migrations /migrations

EXPOSE 8080

CMD ["/api"]
