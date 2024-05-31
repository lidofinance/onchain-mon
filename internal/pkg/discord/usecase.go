package discord

import "context"

//go:generate ./../../../bin/mockery --name Usecase
type Usecase interface {
	SendMessage(ctx context.Context, message string) error
}
