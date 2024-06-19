package env

import (
	"regexp"
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

	NatsDefaultURL string
	NatsStreamName string
	MetricsPrefix  string
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

		readEnvFromShell := viper.GetBool("READ_ENV_FROM_SHELL")
		if !readEnvFromShell {
			if viperErr := viper.ReadInConfig(); viperErr != nil {
				err = viperErr
				return
			}
		}

		var re = regexp.MustCompile(`[ -]`)

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
				NatsDefaultURL:    viper.GetString("NATS_DEFAULT_URL"),
				NatsStreamName:    "FINDINGS",
				MetricsPrefix:     re.ReplaceAllString(viper.GetString("APP_NAME"), `_`),
			},
		}
	})

	return &cfg, err
}
