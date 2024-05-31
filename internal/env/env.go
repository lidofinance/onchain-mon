package env

import (
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	AppConfig AppConfig
}

type AppConfig struct {
	Name              string
	Env               string
	URL               string
	Port              uint
	LogFormat         string
	LogLevel          string
	TelegramBotToken  string
	TelegramChatID    string
	OpsGeniaAPIKey    string
	DiscordWebHookURL string

	RedisURL       string
	RedisPassword  string
	RedisDB        int
	RedisQueueName string
}

var (
	cfg Config

	onceDefaultClient sync.Once
)

func Read(configPath string) (*Config, error) {
	var err error

	onceDefaultClient.Do(func() {
		viper.SetConfigType("env")

		if configPath != "" {
			viper.SetConfigFile(configPath)
		} else {
			viper.AddConfigPath(".")
			viper.SetConfigFile(".env")
		}

		viper.AutomaticEnv()
		if viperErr := viper.ReadInConfig(); viperErr != nil {
			err = viperErr
			return
		}

		cfg = Config{
			AppConfig: AppConfig{
				Name:              viper.GetString("APP_NAME"),
				Env:               viper.GetString("ENV"),
				Port:              viper.GetUint("PORT"),
				LogFormat:         viper.GetString("LOG_FORMAT"),
				LogLevel:          viper.GetString("LOG_LEVEL"),
				TelegramBotToken:  viper.GetString("TELEGRAM_BOT_TOKEN"),
				TelegramChatID:    viper.GetString("TELEGRAM_CHAT_ID"),
				OpsGeniaAPIKey:    viper.GetString("OPSGENIE_API_KEY"),
				DiscordWebHookURL: viper.GetString("DISCORD_WEBHOOK_URL"),

				RedisURL:       viper.GetString("REDIS_URL"),
				RedisPassword:  viper.GetString("REDIS_PASSWORD"),
				RedisDB:        viper.GetInt("REDIS_DB"),
				RedisQueueName: viper.GetString("REDIS_QUEUE_NAME"),
			},
		}
	})

	return &cfg, err
}
