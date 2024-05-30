package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"

	"github.com/sergeyWh1te/go-template/internal/connectors/logger"
	"github.com/sergeyWh1te/go-template/internal/connectors/postgres"
	"github.com/sergeyWh1te/go-template/internal/env"
)

const (
	migrationsFolderPath = "db/migrations"
)

func main() {
	cfg, envErr := env.Read()
	if envErr != nil {
		fmt.Println("Read env error:", envErr.Error())
		return
	}

	log, logErr := logger.New(&cfg.AppConfig)
	if logErr != nil {
		fmt.Println("Logger error:", logErr.Error())
		return
	}

	rootCmd := &cobra.Command{}

	var dbCmd = &cobra.Command{
		Use:   "db",
		Short: "Database tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 0 {
				return fmt.Errorf("no command provided")
			}
			log.Print(args)

			databaseURI := postgres.DatabaseURI(cfg.PgConfig)
			m, err := migrate.New(
				fmt.Sprintf("file://%s", migrationsFolderPath),
				databaseURI,
			)
			if err != nil {
				return err
			}

			defer m.Close()
			switch args[0] {
			case "up":
				log.Print(m.Up())
			case "down":
				log.Print(m.Steps(-1))
			case "drop":
				log.Print(m.Drop())
			case "gm":
				execMigrateCommand(fmt.Sprintf("migrate create -ext sql -dir=%s -seq %s", migrationsFolderPath, args[1]))
			}

			return nil
		},
	}

	rootCmd.AddCommand(dbCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func execMigrateCommand(c string) {
	//nolint:gosec
	cmd := exec.Command("sh", "-c", c)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintln(os.Stderr, cmd.String())
		fmt.Fprintln(os.Stderr, string(out))
		fmt.Fprintln(os.Stderr, err)
		return
	}
	fmt.Fprintln(os.Stdout, string(out))
}
