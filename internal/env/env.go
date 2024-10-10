package env

import (
	"regexp"
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	AppConfig AppConfig
}

type DeliveryServicesConfig struct {
	TelegramBotToken      string
	TelegramErrorsChatID  string
	TelegramUpdatesChatID string
	TelegramAlertsChatID  string
	OpsGenieAPIKey        string
	DiscordWebHookURL     string
}

type AppConfig struct {
	Name      string
	Source    string
	Env       string
	URL       string
	Port      uint
	LogFormat string
	LogLevel  string

	DeliveryConfig      DeliveryServicesConfig
	DeliveryStageConfig DeliveryServicesConfig

	NatsDefaultURL string
	MetricsPrefix  string

	//nolint
	JsonRpcURL string

	BlockTopic       string
	FindingTopic     string
	FortaAlertsTopic string

	RedisURL   string
	QuorumSize uint
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

		blocksTopic := "blocks.mainnet.l1"
		findingTopic := "findings.nats"
		fortaAlertsTopic := "alerts.forta"

		cfg = Config{
			AppConfig: AppConfig{
				Name:      viper.GetString("APP_NAME"),
				Source:    viper.GetString("SOURCE"),
				Env:       viper.GetString("ENV"),
				Port:      viper.GetUint("PORT"),
				LogFormat: viper.GetString("LOG_FORMAT"),
				LogLevel:  viper.GetString("LOG_LEVEL"),

				DeliveryConfig: DeliveryServicesConfig{
					TelegramBotToken:      viper.GetString("TELEGRAM_BOT_TOKEN"),
					TelegramErrorsChatID:  viper.GetString("TELEGRAM_ERRORS_CHAT_ID"),
					TelegramUpdatesChatID: viper.GetString("TELEGRAM_UPDATES_CHAT_ID"),
					TelegramAlertsChatID:  viper.GetString("TELEGRAM_ALERTS_CHAT_ID"),
					OpsGenieAPIKey:        viper.GetString("OPSGENIE_API_KEY"),
					DiscordWebHookURL:     viper.GetString("DISCORD_WEBHOOK_URL"),
				},

				DeliveryStageConfig: DeliveryServicesConfig{
					TelegramBotToken:      viper.GetString("STAGE_TELEGRAM_BOT_TOKEN"),
					TelegramErrorsChatID:  viper.GetString("STAGE_TELEGRAM_ERRORS_CHAT_ID"),
					TelegramUpdatesChatID: viper.GetString("STAGE_TELEGRAM_UPDATES_CHAT_ID"),
					TelegramAlertsChatID:  viper.GetString("STAGE_TELEGRAM_ALERTS_CHAT_ID"),

					OpsGenieAPIKey:    viper.GetString("STAGE_OPSGENIE_API_KEY"),
					DiscordWebHookURL: viper.GetString("STAGE_DISCORD_WEBHOOK_URL"),
				},

				NatsDefaultURL: viper.GetString("NATS_DEFAULT_URL"),
				MetricsPrefix:  re.ReplaceAllString(viper.GetString("APP_NAME"), `_`),
				JsonRpcURL:     viper.GetString("JSON_RPC_URL"),

				BlockTopic:       blocksTopic,
				FindingTopic:     findingTopic,
				FortaAlertsTopic: fortaAlertsTopic,

				RedisURL:   viper.GetString("REDIS_ADDRESS"),
				QuorumSize: viper.GetUint("QUORUM_SIZE"),
			},
		}
	})

	return &cfg, err
}
