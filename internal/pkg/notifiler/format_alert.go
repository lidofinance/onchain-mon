package notifiler

import (
	"fmt"
	"time"

	"github.com/lidofinance/finding-forwarder/generated/databus"
)

func FormatAlert(alert *databus.FindingDtoJson, source string) string {
	var (
		body   string
		footer string
	)

	if len(alert.Description) != 0 {
		body = alert.Description
		footer += "\n\n"
	}

	footer += fmt.Sprintf("Alert Id: %s", alert.AlertId)
	footer += fmt.Sprintf("\nBot name: %s", alert.BotName)
	footer += fmt.Sprintf("\nTeam: %s", alert.Team)

	if alert.BlockNumber != nil {
		footer += fmt.Sprintf("\nBlock number: [%d](https://etherscan.io/block/%d)", *alert.BlockNumber, *alert.BlockNumber)
	}

	if alert.TxHash != nil {
		footer += fmt.Sprintf("\nTx hash: [%s](https://etherscan.io/tx/%s)", shortenHex(*alert.TxHash), *alert.TxHash)
	}

	footer += fmt.Sprintf("\nSource: %s", source)
	footer += fmt.Sprintf("\nServer timestamp: %s", time.Now().Format("15:04:05.000 MST"))
	if alert.BlockTimestamp != nil {
		footer += fmt.Sprintf("\nBlock timestamp:   %s", time.Unix(int64(*alert.BlockTimestamp), 0).Format("15:04:05.000 MST"))
	}

	return fmt.Sprintf("%s%s", body, footer)
}

func shortenHex(input string) string {
	if len(input) <= 10 {
		return input
	}
	return fmt.Sprintf("x%s...%s", input[2:5], input[len(input)-3:])
}
