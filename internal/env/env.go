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
	Name      string
	Source    string
	Env       string
	URL       string
	Port      uint
	LogFormat string
	LogLevel  string

	NatsDefaultURL string
	MetricsPrefix  string

	JsonRpcURL string

	BlockTopic   string
	FindingTopic string

	RedisURL      string
	RedisDB       int
	QuorumSize    uint
	SentryDSN     string
	BlockExplorer string
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

		blockExplorer := viper.GetString("BLOCK_EXPLORER")
		if blockExplorer == "" {
			blockExplorer = `etherscan.io`
		}

		cfg = Config{
			AppConfig: AppConfig{
				Name:      viper.GetString("APP_NAME"),
				Source:    viper.GetString("SOURCE"),
				Env:       viper.GetString("ENV"),
				Port:      viper.GetUint("PORT"),
				LogFormat: viper.GetString("LOG_FORMAT"),
				LogLevel:  viper.GetString("LOG_LEVEL"),

				NatsDefaultURL: viper.GetString("NATS_DEFAULT_URL"),
				MetricsPrefix:  re.ReplaceAllString(viper.GetString("APP_NAME"), `_`),
				JsonRpcURL:     viper.GetString("JSON_RPC_URL"),
				BlockTopic:     viper.GetString("BLOCK_TOPIC"),
				RedisURL:       viper.GetString("REDIS_ADDRESS"),
				RedisDB:        viper.GetInt("REDIS_DB"),
				QuorumSize:     viper.GetUint("QUORUM_SIZE"),
				SentryDSN:      viper.GetString("SENTRY_DSN"),
				BlockExplorer:  blockExplorer,
			},
		}
	})

	return &cfg, err
}
