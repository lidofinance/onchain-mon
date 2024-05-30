package server

import (
	"github.com/jmoiron/sqlx"

	"github.com/sergeyWh1te/go-template/internal/pkg/users"
	userRepo "github.com/sergeyWh1te/go-template/internal/pkg/users/repository"
)

type repository struct {
	User users.Repository
}

// nolint
func Repository(db *sqlx.DB) *repository {
	return &repository{
		User: userRepo.New(db),
	}
}
