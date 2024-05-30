package users

import (
	"context"

	"github.com/sergeyWh1te/go-template/internal/pkg/users/entity"
)

//go:generate ./../../../bin/mockery --name Repository
type Repository interface {
	Get(ctx context.Context, ID int64) (*entity.User, error)
	Create(ctx context.Context) (*int64, error)
}
