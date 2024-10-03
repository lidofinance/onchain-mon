package notifiler

import (
	"context"

	"github.com/lidofinance/finding-forwarder/generated/databus"
	"github.com/lidofinance/finding-forwarder/generated/forta/models"
)

type FindingSender interface {
	SendFinding(ctx context.Context, alert *databus.FindingDtoJson) error
	SendAlert(ctx context.Context, alert *models.Alert) error
}
