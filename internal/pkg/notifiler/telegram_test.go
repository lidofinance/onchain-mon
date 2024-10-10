package notifiler

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/env"
)

// Helper function to create pointers for strings
func stringPtr(s string) *string {
	return &s
}

// Helper function to create pointers for ints
func intPtr(i int) *int {
	return &i
}

const ParadiseLost = `
Of Mans First Disobedience, and the Fruit
Of that Forbidden Tree, whose mortal taste
Brought Death into the World, and all our woe,
With loss of Eden, till one greater Man
Restore us, and regain the blissful Seat,

Sing Heav'nly Muse, that on the secret top
Of Oreb, or of Sinai, didst inspire
That Shepherd, who first taught the chosen Seed,
In the Beginning how the Heav'ns and Earth

Rose out of Chaos: Or if Sion Hill
Delight thee more, and Siloa's Brook that flow'd
Fast by the Oracle of God; I thence
Invoke thy aid to my adventurous Song,
That with no middle flight intends to soar

Above th' Aonian Mount, while it pursues
Things unattempted yet in Prose or Rhime.
And chiefly Thou O Spirit, that dost prefer
Before all Temples th' upright heart and pure,

Instruct me, for Thou know'st; Thou from the first
Wast present, and with mighty wings outspread
Dove-like satst brooding on the vast Abyss
And mad'st it pregnant: What in me is dark

Illumin, what is low raise and support;
That to the highth of this great Argument
I may assert Eternal Providence,
And justifie the wayes of God to men.

Say first, for Heav'n hides nothing from thy view
Nor the deep Tract of Hell, say first what cause
Mov'd our Grand Parents in that happy State,
Favour'd of Heav'n so highly, to fall off

From thir Creator, and transgress his Will
For one restraint, Lords of the World besides?
Who first seduc'd them to that foul revolt?
Th' infernal Serpent; he it was, whose guile
Stird up with Envy and Revenge, deceiv'd

The Mother of Mankind, what time his Pride
Had cast him out from Heav'n, with all his Host
Of Rebel Angels, by whose aid aspiring
To set himself in Glory above his Peers,

He trusted to have equal'd the most High,
If he oppos'd; and with ambitious aim
Against the Throne and Monarchy of God
Rais'd impious War in Heav'n and Battel proud

With vain attempt. Him the Almighty Power
Hurld headlong flaming from th' Ethereal Skie
With hideous ruine and combustion down
To bottomless perdition, there to dwell

In Adamantine Chains and penal Fire,
Who durst defie th' Omnipotent to Arms.
Nine times the Space that measures Day and Night
To mortal men, he with his horrid crew

Lay vanquisht, rowling in the fiery Gulfe
Confounded though immortal: But his doom
Reserv'd him to more wrath; for now the thought
Both of lost happiness and lasting pain

Torments him; round he throws his baleful eyes
That witness'd huge affliction and dismay
Mixt with obdurate pride and stedfast hate:
At once as far as Angels kenn he views

The dismal Situation waste and wilde,
A Dungeon horrible, on all sides round
As one great Furnace flam'd, yet from those flames
No light, but rather darkness visible

Serv'd onely to discover sights of woe,
Regions of sorrow, doleful shades, where peace
And rest can never dwell, hope never comes
That comes to all; but torture without end

Still urges, and a fiery Deluge, fed
With ever-burning Sulfur unconsum'd:
Such place Eternal Justice had prepar'd
For those rebellious, here thir Prison ordain'd

In utter darkness, and thir portion set
As far remov'd from God and light of Heav'n
As from the Center thrice to th' utmost Pole.
O how unlike the place from whence they fell!

There the companions of his fall, o'rewhelm'd
With Floods and Whirlwinds of tempestuous fire,
He soon discerns, and weltring by his side
One next himself in power, and next in crime,

Long after known in Palestine, and nam'd
Beelzebub. To whom th' Arch-Enemy,
And thence in Heav'n call'd Satan, with bold words
Breaking the horrid silence thus began.

If thou beest he; But O how fall'n! how chang'd
From him, who in the happy Realms of Light
Cloth'd with transcendent brightness didst out-shine
Myriads though bright: If he Whom mutual league,

United thoughts and counsels, equal hope
And hazard in the Glorious Enterprise,
Joynd with me once, now misery hath joynd
In equal ruin: into what Pit thou seest

From what highth fall'n, so much the stronger prov'd
He with his Thunder: and till then who knew
The force of those dire Arms? yet not for those,
Nor what the Potent Victor in his rage

Can else inflict, do I repent or change,
Though chang'd in outward luster; that fixt mind
And high disdain, from sense of injur'd merit,
That with the mightiest rais'd me to contend,
And to the fierce contention brought along

Innumerable force of Spirits arm'd
That durst dislike his reign, and me preferring,
His utmost power with adverse power oppos'd
In dubious Battel on the Plains of Heav'n,
And shook his throne. What though the field be lost?

All is not lost; the unconquerable Will,
And study of revenge, immortal hate,
And courage never to submit or yield:
And what is else not to be overcome?

That Glory never shall his wrath or might
Extort from me. To bow and sue for grace
With suppliant knee, and deifie his power,
Who from the terrour of this Arm so late

Doubted his Empire, that were low indeed,
That were an ignominy and shame beneath
This downfall; since by Fate the strength of Gods
And this Empyreal substance cannot fail,

Since through experience of this great event
In Arms not worse, in foresight much advanc't,
We may with more successful hope resolve
To wage by force or guile eternal Warr

Irreconcileable, to our grand Foe,
Who now triumphs, and in th' excess of joy
Sole reigning holds the Tyranny of Heav'n.

Alert Id: LIDO-TOKEN-REBASED
Bot name: FF-telegram-unit-test
Team: Protocol
Block number: [20884540](https://etherscan.io/block/20884540/)
Tx hash: [x714...bdf](https://etherscan.io/tx/0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf/)
Source: local
Server timestamp: 18:36:55.152 MSK
Block  timestamp: 17:20:36.000 MSK`

func Test_SendFinding(t *testing.T) {
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
				botToken:     cfg.AppConfig.DeliveryConfig.TelegramBotToken,
				chatID:       cfg.AppConfig.DeliveryConfig.TelegramErrorsChatID,
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
				botToken:     cfg.AppConfig.DeliveryConfig.TelegramBotToken,
				chatID:       cfg.AppConfig.DeliveryConfig.TelegramErrorsChatID,
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
				botToken:     cfg.AppConfig.DeliveryConfig.TelegramBotToken,
				chatID:       cfg.AppConfig.DeliveryConfig.TelegramErrorsChatID,
				httpClient:   &http.Client{},
				metricsStore: metricsStore,
			},
			args: args{
				ctx: context.TODO(),
				alert: &databus.FindingDtoJson{
					Name:           "ℹ️ Lido: Token rebased",
					Description:    ParadiseLost,
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
				message: ParadiseLost,
			},
			want: `
Of Mans First Disobedience, and the Fruit
Of that Forbidden Tree, whose mortal taste
Brought Death into the World, and all our woe,
With loss of Eden, till one greater Man
Restore us, and regain the blissful Seat,

Sing Heav'nly Muse, that on the secret top
Of Oreb, or of Sinai, didst inspire
That Shepherd, who first taught the chosen Seed,
In the Beginning how the Heav'ns and Earth

Rose out of Chaos: Or if Sion Hill
Delight thee more, and Siloa's Brook that flow'd
Fast by the Oracle of God; I thence
Invoke thy aid to my adventurous Song,
That with no middle flight intends to soar

Above th' Aonian Mount, while it pursues
Things unattempted yet in Prose or Rhime.
And chiefly Thou O Spirit, that dost prefer
Before all Temples th' upright heart and pure,

Instruct me, for Thou know'st; Thou from the first
Wast present, and with mighty wings outspread
Dove-like satst brooding on the vast Abyss
And mad'st it pregnant: What in me is dark

Illumin, what is low raise and support;
That to the highth of this great Argument
I may assert Eternal Providence,
And justifie the wayes of God to men.

Say first, for Heav'n hides nothing from thy view
Nor the deep Tract of Hell, say first what cause
Mov'd our Grand Parents in that happy State,
Favour'd of Heav'n so highly, to fall off

From thir Creator, and transgress his Will
For one restraint, Lords of the World besides?
Who first seduc'd them to that foul revolt?
Th' infernal Serpent; he it was, whose guile
Stird up with Envy and Revenge, deceiv'd

The Mother of Mankind, what time his Pride
Had cast him out from Heav'n, with all his Host
Of Rebel Angels, by whose aid aspiring
To set himself in Glory above his Peers,

He trusted to have equal'd the most High,
If he oppos'd; and with ambitious aim
Against the Throne and Monarchy of God
Rais'd impious War in Heav'n and Battel proud

With vain attempt. Him the Almighty Power
Hurld headlong flaming from th' Ethereal Skie
With hideous ruine and combustion down
To bottomless perdition, there to dwell

In Adamantine Chains and penal Fire,
Who durst defie th' Omnipotent to Arms.
Nine times the Space that measures Day and Night
To mortal men, he with his horrid crew

Lay vanquisht, rowling in the fiery Gulfe
Confounded though immortal: But his doom
Reserv'd him to more wrath; for now the thought
Both of lost happiness and lasting pain

Torments him; round he throws his baleful eyes
That witness'd huge affliction and dismay
Mixt with obdurate pride and stedfast hate:
At once as far as Angels kenn he views

The dismal Situation waste and wilde,
A Dungeon horrible, on all sides round
As one great Furnace flam'd, yet from those flames
No light, but rather darkness visible

Serv'd onely to discover sights of woe,
Regions of sorrow, doleful shades, where peace
And rest can never dwell, hope never comes
That comes to all; but torture without end

Still urges, and a fiery Deluge, fed
With ever-burning Sulfur unconsum'd:
Such place Eternal Justice had prepar'd
For those rebellious, here thir Prison ordain'd

In utter darkness, and thir portion set
As far remov'd from God and light of Heav'n
As from the Center thrice to th' utmost Pole.
O how unlike the place from whence they fell!

There the companions of his fall, o'rewhelm'd
With Floods and Whirlwinds of tempestuous fire,
He soon discerns, and weltring by his side
One next himself in power, and next in crime,

Long after known in Palestine, and nam'd
Beelzebub. To whom th' Arch-Enemy,
And thence in Heav'n call'd Satan, with bold words
Breaking the horrid silence thus began.

If thou beest he; But O how fall'n! how chang'd
From him, who in the happy Realms of Light
Cloth'd with transcendent brightness did
...

*Warn: Msg >=4096, pls review description message*
Alert Id: LIDO-TOKEN-REBASED
Bot name: FF-telegram-unit-test
Team: Protocol
Block number: [20884540](https://etherscan.io/block/20884540/)
Tx hash: [x714...bdf](https://etherscan.io/tx/0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf/)
Source: local
Server timestamp: 18:36:55.152 MSK
Block  timestamp: 17:20:36.000 MSK`,
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
