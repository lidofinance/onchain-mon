package postgres

import (
	"fmt"
	"sync"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/sergeyWh1te/go-template/internal/env"
)

var (
	db                *sqlx.DB
	onceDefaultClient sync.Once
)

const (
	MaxOpenConns = 25
	MaxIdleConns = 60 * int(time.Second)
)

func DatabaseURI(config env.PgConfig) string {
	return fmt.Sprintf(`postgres://%s:%s@%s:%d/%s?sslmode=%s`,
		config.Username, config.Password, config.Host, config.Port, config.Database, config.SslMode)
}

func Connect(config env.PgConfig) (*sqlx.DB, error) {
	conf, parseErr := pgx.ParseConfig(
		fmt.Sprintf(`host=%s port=%d user=%s password=%s dbname=%s sslmode=%s simple_protocol=%t`,
			config.Host, config.Port, config.Username, config.Password, config.Database, config.SslMode, true),
	)

	if parseErr != nil {
		return nil, parseErr
	}

	conf.RuntimeParams = map[string]string{
		"standard_conforming_strings": "on",
	}

	var err error
	onceDefaultClient.Do(func() {
		db = sqlx.NewDb(stdlib.OpenDB(*conf), "pgx")

		if err = db.Ping(); err != nil {
			return
		}

		db.SetMaxOpenConns(MaxOpenConns)
		db.SetMaxIdleConns(MaxIdleConns)
	})

	return db, err
}
