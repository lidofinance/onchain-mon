# Build stage
FROM golang:1.20.5-alpine as builder

WORKDIR /go/src/app

COPY . .

# Собираем приложение
RUN go build -o ./bin/main ./cmd/service
RUN go build -o ./bin/migrator ./cmd/migrator

# Run stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /go/src/app/bin ./bin
COPY --from=builder /go/src/app/db/migrations ./db/migrations

EXPOSE 8080

CMD ["./bin/main"]