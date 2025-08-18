package notifiler

import (
	"context"
	"errors"

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

var ErrRateLimited = errors.New("reach request limit")
