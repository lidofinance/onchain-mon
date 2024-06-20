# Build stage
FROM golang:1.22.3-alpine as builder

WORKDIR /go/src/app
COPY . .

RUN go build -ldflags="-X github.com/lidofinance/finding-forwarder/internal/connectors/metrics.Commit=$(git rev-parse HEAD)" -o ./bin/main ./cmd/service
RUN go build -ldflags="-X github.com/lidofinance/finding-forwarder/internal/connectors/metrics.Commit=$(git rev-parse HEAD)" -o ./bin/worker ./cmd/worker

# Run stage
FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates

COPY --from=builder /go/src/app/bin .

USER nobody
CMD ["/app/main"]
