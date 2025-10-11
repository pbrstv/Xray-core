FROM golang:1.25.1-alpine3.22 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o xray ./main

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && \
    adduser -D xray && \
    mkdir -p /etc/xray
COPY --from=builder --chown=xray:xray /app/xray /usr/local/bin/xray
USER xray
CMD ["/app/xray", "-c", "/etc/xray/config.json"]
