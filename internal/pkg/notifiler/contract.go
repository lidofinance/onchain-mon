package notifiler

import (
	"context"

	"github.com/lidofinance/finding-forwarder/generated/databus"
)

type FindingSender interface {
	SendFinding(ctx context.Context, alert *databus.FindingDtoJson) error
}
