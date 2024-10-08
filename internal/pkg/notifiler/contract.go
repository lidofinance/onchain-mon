package notifiler

import (
	"context"

	"github.com/lidofinance/onchain-mon/generated/databus"
)

type FindingSender interface {
	SendFinding(ctx context.Context, alert *databus.FindingDtoJson) error
}
