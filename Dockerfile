#Dockerfile

FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o drunaServer ./cmd/main.go


FROM alpine:3.22

ARG TARGETARCH=amd64
ARG MIGRATE_VERSION=v4.18.1

RUN apk add --no-cache curl \
    && curl -fsSL "https://github.com/golang-migrate/migrate/releases/download/${MIGRATE_VERSION}/migrate.linux-${TARGETARCH}.tar.gz" \
    | tar xz -C /usr/local/bin migrate \
    && apk del curl \
    && addgroup -S druna && adduser -S -G druna -h /app druna

WORKDIR /app
COPY --from=builder /app/drunaServer .
COPY configs/config.docker.yaml ./configs/config.yaml
COPY migrations /migrations
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

USER druna

ENTRYPOINT ["/docker-entrypoint.sh"]
