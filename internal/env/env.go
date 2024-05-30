package env

import (
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	AppConfig AppConfig
}

type AppConfig struct {
	Name      string
	Env       string
	URL       string
	Port      uint
	LogFormat string
	LogLevel  string
}

var (
	cfg Config

	onceDefaultClient sync.Once
)

func Read() (*Config, error) {
	var err error

	onceDefaultClient.Do(func() {
		viper.SetConfigType("env")
		viper.AddConfigPath(".")
		viper.SetConfigFile(".env")

		viper.AutomaticEnv()
		if viperErr := viper.ReadInConfig(); err != nil {
			if _, ok := viperErr.(viper.ConfigFileNotFoundError); !ok {
				err = viperErr
				return
			}
		}

		cfg = Config{
			AppConfig: AppConfig{
				Name:      viper.GetString("APP_NAME"),
				Env:       viper.GetString("ENV"),
				URL:       viper.GetString("app.url"),
				Port:      viper.GetUint("PORT"),
				LogFormat: viper.GetString("LOG_FORMAT"),
				LogLevel:  viper.GetString("LOG_LEVEL"),
			},
		}
	})

	return &cfg, err
}
