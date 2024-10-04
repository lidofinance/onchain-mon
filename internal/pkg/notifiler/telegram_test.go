package notifiler

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/finding-forwarder/generated/databus"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	"github.com/lidofinance/finding-forwarder/internal/env"
)

// Helper function to create pointers for strings
func stringPtr(s string) *string {
	return &s
}

// Helper function to create pointers for ints
func intPtr(i int) *int {
	return &i
}

const EugineOnegin = `
«Мой дядя самых честных правил,
Когда не в шутку занемог,
Он уважать себя заставил
И лучше выдумать не мог.
Его пример другим наука;
Но, боже мой, какая скука
С больным сидеть и день и ночь,
Не отходя ни шагу прочь!
Какое низкое коварство
Полуживого забавлять,
Ему подушки поправлять,
Печально подносить лекарство,
Вздыхать и думать про себя:
Когда же черт возьмет тебя!»

II
Так думал молодой повеса,
Летя в пыли на почтовых,
Всевышней волею Зевеса
Наследник всех своих родных.
Друзья Людмилы и Руслана!
С героем моего романа
Без предисловий, сей же час
Позвольте познакомить вас:
Онегин, добрый мой приятель,
Родился на брегах Невы,
Где, может быть, родились вы
Или блистали, мой читатель;
Там некогда гулял и я:
Но вреден север для меня 1.

III
Служив отлично благородно,
Долгами жил его отец,
Давал три бала ежегодно
И промотался наконец.
Судьба Евгения хранила:
Сперва Madame за ним ходила,
Потом Monsieur ее сменил.
Ребенок был резов, но мил.
Monsieur l'Abbé, француз убогой,
Чтоб не измучилось дитя,
Учил его всему шутя,
Не докучал моралью строгой,
Слегка за шалости бранил
И в Летний сад гулять водил.

IV
Когда же юности мятежной
Пришла Евгению пора,
Пора надежд и грусти нежной,
Monsieur прогнали со двора.
Вот мой Онегин на свободе;
Острижен по последней моде,
Как dandy 2 лондонский одет —
И наконец увидел свет.
Он по-французски совершенно
Мог изъясняться и писал;
Легко мазурку танцевал
И кланялся непринужденно;
Чего ж вам больше? Свет решил,
Что он умен и очень мил.

V
Мы все учились понемногу
Чему-нибудь и как-нибудь,
Так воспитаньем, слава богу,
У нас немудрено блеснуть.
Онегин был по мненью многих
(Судей решительных и строгих)
Ученый малый, но педант:
Имел он счастливый талант
Без принужденья в разговоре
Коснуться до всего слегка,
С ученым видом знатока
Хранить молчанье в важном споре
И возбуждать улыбку дам
Огнем нежданных эпиграмм.

VI
Латынь из моды вышла ныне:
Так, если правду вам сказать,
Он знал довольно по-латыне,
Чтоб эпиграфы разбирать,
Потолковать об Ювенале,
В конце письма поставить vale,
Да помнил, хоть не без греха,
Из Энеиды два стиха.
Он рыться не имел охоты
В хронологической пыли
Бытописания земли:
Но дней минувших анекдоты
От Ромула до наших дней
Хранил он в памяти своей.

VII
Высокой страсти не имея
Для звуков жизни не щадить,
Не мог он ямба от хорея,
Как мы ни бились, отличить.
Бранил Гомера, Феокрита;
Зато читал Адама Смита
И был глубокой эконом,
То есть умел судить о том,
Как государство богатеет,
И чем живет, и почему
Не нужно золота ему,
Когда простой продукт имеет.
Отец понять его не мог
И земли отдавал в залог.

VIII
Всего, что знал еще Евгений,
Пересказать мне недосуг;
Но в чем он истинный был гений,
Что знал он тверже всех наук,
Что было для него измлада
И труд, и мука, и отрада,
Что занимало целый день
Его тоскующую лень, —
Была наука страсти нежной,
Которую воспел Назон,
За что страдальцем кончил он
Свой век блестящий и мятежный
В Молдавии, в глуши степей,
Вдали Италии своей

Alert Id: LIDO-TOKEN-REBASED
Bot name: FF-telegram-unit-test
Team: Protocol
Block number: [20884540](https://etherscan.io/block/20884540/)
Tx hash: [x714...bdf](https://etherscan.io/tx/0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf/)
Source: local
Server timestamp: 18:36:55.152 MSK
Block timestamp:   17:20:36.000 MSK`

func Test_SendFinfing(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}

	promRegistry := prometheus.NewRegistry()
	metricsStore := metrics.New(promRegistry, cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	type fields struct {
		botToken     string
		chatID       string
		httpClient   *http.Client
		metricsStore *metrics.Store
	}
	type args struct {
		ctx   context.Context
		alert *databus.FindingDtoJson
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Send_Test_Message_to_telegram",
			fields: fields{
				botToken:     cfg.AppConfig.TelegramBotToken,
				chatID:       cfg.AppConfig.TelegramErrorsChatID,
				httpClient:   &http.Client{},
				metricsStore: metricsStore,
			},
			args: args{
				ctx: context.TODO(),
				alert: &databus.FindingDtoJson{
					Name: `ℹ️ Lido: Token rebased`,
					Description: `
Withdrawals info:
 requests count:    4302
 withdrawn stETH:   174541.1742
 finalized stETH:   174541.1742 4302
 unfinalized stETH: 0.0000   0
 claimed ether:     142576.2152 853
 unclaimed ether:   31964.9590   3449
`,
					Severity:       databus.SeverityLow,
					AlertId:        `LIDO-TOKEN-REBASED`,
					BlockTimestamp: intPtr(1727965236),
					BlockNumber:    intPtr(20884540),
					TxHash:         stringPtr("0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"),
					BotName:        `FF-telegram-unit-test`,
					Team:           `Protocol`,
				},
			},
			wantErr: false,
		},
		{
			name: "Send_Test_Message_to_telegram",
			fields: fields{
				botToken:     cfg.AppConfig.TelegramBotToken,
				chatID:       cfg.AppConfig.TelegramErrorsChatID,
				httpClient:   &http.Client{},
				metricsStore: metricsStore,
			},
			args: args{
				ctx: context.TODO(),
				alert: &databus.FindingDtoJson{
					Name:           "ℹ️ #l2_arbitrum Arbitrum digest",
					Description:    "L1 token rate: 1.1808\nBridge balances:\n\tLDO:\n\t\tL1: 1231218.4603 LDO\n\t\tL2: 1230730.9530 LDO\n\t\n\twstETH:\n\t\tL1: 84477.0663 wstETH\n\t\tL2: 81852.1638 wstETH\n\nWithdrawals:\n\twstETH: 1664.1363 (in 5 transactions)",
					Severity:       databus.SeverityInfo,
					AlertId:        `DIGEST`,
					BlockTimestamp: intPtr(1727965236),
					BlockNumber:    intPtr(20884540),
					TxHash:         stringPtr("0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"),
					BotName:        `Test`,
					Team:           `Protocol`,
				},
			},
			wantErr: false,
		},
		{
			name: "Send_4096_symbol_to_telegram",
			fields: fields{
				botToken:     cfg.AppConfig.TelegramBotToken,
				chatID:       cfg.AppConfig.TelegramErrorsChatID,
				httpClient:   &http.Client{},
				metricsStore: metricsStore,
			},
			args: args{
				ctx: context.TODO(),
				alert: &databus.FindingDtoJson{
					Name:           "ℹ️ Lido: Token rebased",
					Description:    EugineOnegin,
					Severity:       databus.SeverityInfo,
					AlertId:        `DIGEST`,
					BlockTimestamp: intPtr(1727965236),
					BlockNumber:    intPtr(20884540),
					TxHash:         stringPtr("0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"),
					BotName:        `Test`,
					Team:           `Protocol`,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := NewTelegram(
				tt.fields.botToken,
				tt.fields.chatID,
				tt.fields.httpClient,
				metricsStore,
				`local`,
			)
			if err := u.SendFinding(tt.args.ctx, tt.args.alert); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTruncateMessageWithAlertID(t *testing.T) {
	type args struct {
		message string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			args: args{
				message: EugineOnegin,
			},
			want: `
«Мой дядя самых честных правил,
Когда не в шутку занемог,
Он уважать себя заставил
И лучше выдумать не мог.
Его пример другим наука;
Но, боже мой, какая скука
С больным сидеть и день и ночь,
Не отходя ни шагу прочь!
Какое низкое коварство
Полуживого забавлять,
Ему подушки поправлять,
Печально подносить лекарство,
Вздыхать и думать про себя:
Когда же черт возьмет тебя!»

II
Так думал молодой повеса,
Летя в пыли на почтовых,
Всевышней волею Зевеса
Наследник всех своих родных.
Друзья Людмилы и Руслана!
С героем моего романа
Без предисловий, сей же час
Позвольте познакомить вас:
Онегин, добрый мой приятель,
Родился на брегах Невы,
Где, может быть, родились вы
Или блистали, мой читатель;
Там некогда гулял и я:
Но вреден север для меня 1.

III
Служив отлично благородно,
Долгами жил его отец,
Давал три бала ежегодно
И промотался наконец.
Судьба Евгения хранила:
Сперва Madame за ним ходила,
Потом Monsieur ее сменил.
Ребенок был резов, но мил.
Monsieur l'Abbé, француз убогой,
Чтоб не измучилось дитя,
Учил его всему шутя,
Не докучал моралью строгой,
Слегка за шалости бранил
И в Летний сад гулять водил.

IV
Когда же юности мятежной
Пришла Евгению пора,
Пора надежд и грусти нежной,
Monsieur прогнали со двора.
Вот мой Онегин на свободе;
Острижен по последней моде,
Как dandy 2 лондонский одет —
И наконец увидел свет.
Он по-французски совершенно
Мог изъясняться и писал;
Легко мазурку танцевал
И кланялся непринужденно;
Чего ж вам больше? Свет решил,
Что он умен и очень мил.

V
Мы все учились понемногу
Чему-нибудь и как-нибудь,
Так воспитаньем, слава богу,
У нас немудрено блеснуть.
Онегин был по мненью многих
(Судей решительных и строгих)
Ученый малый, но педант:
Имел он счастливый талант
Без принужденья в разговоре
Коснуться до всего слегка,
С ученым видом знатока
Хранить молчанье в важном споре
И возбуждать улыбку дам
Огнем нежданных эпиграмм.

VI
Латынь из моды вышла ныне:
Так, если правду вам сказать,
Он знал довольно по-латыне,
Чтоб эпиграфы разбирать,
Потолковать об Ювенале,
В конце письма поставить vale,
Да помнил, хоть не без греха,
Из Энеиды два стиха.
Он ры
...

*Warn: Msg >=4096, pls review description message*
Alert Id: LIDO-TOKEN-REBASED
Bot name: FF-telegram-unit-test
Team: Protocol
Block number: [20884540](https://etherscan.io/block/20884540/)
Tx hash: [x714...bdf](https://etherscan.io/tx/0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf/)
Source: local
Server timestamp: 18:36:55.152 MSK
Block timestamp:   17:20:36.000 MSK`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TruncateMessageWithAlertID(tt.args.message, maxTelegramMessageLength, warningTelegramMessage); got != tt.want {
				t.Errorf("TruncateMessageWithAlertID() = %v, want %v", got, tt.want)
			}
		})
	}
}
