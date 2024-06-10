# Build stage
FROM golang:1.22.3-alpine as builder

WORKDIR /go/src/app
COPY . .

RUN go build -o ./bin/main ./cmd/service

# Run stage
FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates

COPY --from=builder /go/src/app/bin .

USER nobody
CMD ["/app/main"]
