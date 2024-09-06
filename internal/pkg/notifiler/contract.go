package notifiler

import (
	"context"
	"github.com/lidofinance/finding-forwarder/generated/forta/models"
	"github.com/lidofinance/finding-forwarder/generated/proto"
)

type FindingSender interface {
	SendFinding(ctx context.Context, alert *proto.Finding) error
	SendAlert(ctx context.Context, alert *models.Alert) error
}
