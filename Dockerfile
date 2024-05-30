# Build stage
FROM golang:1.22.3-alpine as builder

WORKDIR /go/src/app

COPY . .

# Собираем приложение
RUN go build -o ./bin/main ./cmd/service

# Run stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /go/src/app/bin ./bin

EXPOSE 8080

CMD ["./bin/main"]