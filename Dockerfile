FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/panel ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs
WORKDIR /app

COPY --from=builder /out/panel /app/panel
COPY config /app/config
COPY templates /app/templates
COPY static /app/static

ENV PANEL_CONFIG=/app/config/config.yaml
EXPOSE 8080

CMD ["/app/panel"]
