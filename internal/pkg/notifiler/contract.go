package notifiler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

type FindingSender interface {
	SendFinding(ctx context.Context, alert *databus.FindingDtoJson, quorumBy string) error
	GetType() registry.NotificationChannel
	GetChannelID() string
	GetRedisStreamName() string
	GetRedisConsumerGroupName() string
}

var ErrRateLimited = errors.New("rate limit reached")

type RateLimitedError struct {
	ResetAfter time.Duration
	Err        error
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("%v (retry after %s)", e.Err, e.ResetAfter)
}

func (e *RateLimitedError) Unwrap() error {
	return e.Err
}

var ErrMarkdownParse = errors.New("markdown parse")
