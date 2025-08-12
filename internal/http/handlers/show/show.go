package show

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/lidofinance/onchain-mon/internal/env"
)

type ConsumerInfo struct {
	ConsumerName string
	Type         string
	ChannelDesc  string
	Severities   []string
	ByQuorum     bool
	Team         string
	Bot          string
	Subject      string
}

type TemplateData struct {
	Consumers []ConsumerInfo
}

type handler struct {
	cfg *env.NotificationConfig
}

func New(cfg *env.NotificationConfig) *handler {
	return &handler{
		cfg: cfg,
	}
}

func (h *handler) Handler(w http.ResponseWriter, _ *http.Request) {
	var consumerInfos []ConsumerInfo

	for _, consumer := range h.cfg.Consumers {
		for _, subject := range consumer.Subjects {
			parts := strings.Split(subject, ".")
			if len(parts) < 3 {
				continue
			}

			teamName := parts[1]
			botName := parts[2]

			generatedName := fmt.Sprintf("%s_%s_%s", teamName, consumer.ConsumerName, botName)

			var channelDesc string
			switch consumer.Type {
			case "Telegram":
				for _, ch := range h.cfg.TelegramChannels {
					if ch.ID == consumer.ChannelID {
						channelDesc = ch.Description
					}
				}
			case "Discord":
				for _, ch := range h.cfg.DiscordChannels {
					if ch.ID == consumer.ChannelID {
						channelDesc = ch.Description
					}
				}
			case "OpsGenie":
				for _, ch := range h.cfg.OpsGenieChannels {
					if ch.ID == consumer.ChannelID {
						channelDesc = ch.Description
					}
				}
			}

			consumerInfo := ConsumerInfo{
				ConsumerName: generatedName,
				Type:         consumer.Type,
				ChannelDesc:  channelDesc,
				Severities:   consumer.Severities,
				ByQuorum:     consumer.ByQuorum,
				Team:         teamName,
				Bot:          botName,
				Subject:      subject,
			}

			consumerInfos = append(consumerInfos, consumerInfo)
		}
	}

	data := TemplateData{
		Consumers: consumerInfos,
	}

	tmpl, err := template.ParseFiles("./web/templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
