package notifiler

import (
	"context"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

type FindingSender interface {
	SendFinding(ctx context.Context, alert *databus.FindingDtoJson) error
	GetType() registry.NotificationChannel
	GetChannelID() string
	GetRedisStreamName() string
}
