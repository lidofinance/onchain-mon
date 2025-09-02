# Build stage
FROM golang:1.25.0-alpine AS builder

WORKDIR /go/src/app
COPY . .

RUN apk add git=2.49.1-r0

RUN go build -ldflags="-X github.com/lidofinance/onchain-mon/internal/connectors/metrics.Commit=$(git rev-parse HEAD)" -o ./bin/feeder ./cmd/feeder
RUN go build -ldflags="-X github.com/lidofinance/onchain-mon/internal/connectors/metrics.Commit=$(git rev-parse HEAD)" -o ./bin/forwarder ./cmd/forwarder

# Run stage
FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates

COPY --from=builder /go/src/app/bin .
COPY --from=builder /go/src/app/web /app/web/

USER nobody
