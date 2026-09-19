FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /pgadmin-go ./cmd/pgadmin-server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates postgresql-client
COPY --from=builder /pgadmin-go /usr/local/bin/pgadmin-go
COPY config/config.yaml /config/config.yaml
COPY web /web
COPY internal/api/templates /internal/api/templates
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/pgadmin-go", "-config", "/config/config.yaml"]
