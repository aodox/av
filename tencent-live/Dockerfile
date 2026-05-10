FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o tencent-live ./cmd/server

FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/tencent-live .
COPY --from=builder /app/config/config.yaml ./config/

RUN mkdir -p /app/logs

EXPOSE 8080

CMD ["./tencent-live", "-config", "./config/config.yaml"]
