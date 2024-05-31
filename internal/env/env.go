package env

import (
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	AppConfig AppConfig
}

type AppConfig struct {
	Name             string
	Env              string
	URL              string
	Port             uint
	LogFormat        string
	LogLevel         string
	TelegramBotToken string
	TelegramChatID   string
}

var (
	cfg Config

	onceDefaultClient sync.Once
)

func Read(configPath string) (*Config, error) {
	var err error

	onceDefaultClient.Do(func() {
		viper.SetConfigType("env")

		if len(configPath) != 0 {
			viper.SetConfigFile(configPath)
		} else {
			viper.AddConfigPath(".")
			viper.SetConfigFile(".env")
		}

		viper.AutomaticEnv()
		if viperErr := viper.ReadInConfig(); err != nil {
			if _, ok := viperErr.(viper.ConfigFileNotFoundError); !ok {
				err = viperErr
				return
			}
		}

		cfg = Config{
			AppConfig: AppConfig{
				Name:             viper.GetString("APP_NAME"),
				Env:              viper.GetString("ENV"),
				URL:              viper.GetString("app.url"),
				Port:             viper.GetUint("PORT"),
				LogFormat:        viper.GetString("LOG_FORMAT"),
				LogLevel:         viper.GetString("LOG_LEVEL"),
				TelegramBotToken: viper.GetString("TELEGRAM_BOT_TOKEN"),
				TelegramChatID:   viper.GetString("TELEGRAM_CHAT_ID"),
			},
		}
	})

	return &cfg, err
}
