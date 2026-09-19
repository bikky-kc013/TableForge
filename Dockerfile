FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /tableforge ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates postgresql-client
COPY --from=builder /tableforge /usr/local/bin/tableforge
COPY config/config.yaml /config/config.yaml
COPY web /web
COPY internal/api/templates /internal/api/templates
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/tableforge", "-config", "/config/config.yaml"]
