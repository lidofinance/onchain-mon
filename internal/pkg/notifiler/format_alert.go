package notifiler

import (
	"fmt"
	"strings"
	"time"

	"github.com/lidofinance/onchain-mon/generated/databus"
)

// Now returns the quorum timestamp printed in the alert footer. Tests replace
// it to make the rendered output deterministic.
var Now = time.Now

func FormatAlert(alert *databus.FindingDtoJson, source, blockExplorer string) string {
	var (
		body   string
		footer string
	)

	if alert.Description != "" {
		body = alert.Description
		footer += "\n"
	}

	quorumTime := Now()

	quorumAt := quorumTime.Format("15:04:05.000 MST")
	if alert.BlockTimestamp != nil {
		eventToQuorumSecs := int(quorumTime.Unix()) - *alert.BlockTimestamp
		quorumAt += fmt.Sprintf(" (+%ds)", eventToQuorumSecs)
	}

	footer += fmt.Sprintf("\n%s | %s | %s by %s",
		alert.BotName, alert.AlertId, quorumAt, source,
	)

	var links []string
	if alert.BlockNumber != nil {
		links = append(links, fmt.Sprintf("[%d](https://%s/block/%d/)", *alert.BlockNumber, blockExplorer, *alert.BlockNumber))
	}
	if alert.TxHash != nil {
		links = append(links, fmt.Sprintf("[%s](https://%s/tx/%s/)", shortenHex(*alert.TxHash), blockExplorer, *alert.TxHash))
	}
	if len(links) > 0 {
		footer += "\n" + strings.Join(links, " | ")
	}

	return fmt.Sprintf("%s%s", body, footer)
}

func shortenHex(input string) string {
	const dontHideInputLength = 5
	if len(input) <= dontHideInputLength {
		return input
	}
	return fmt.Sprintf("0x%s...%s", input[2:5], input[len(input)-3:])
}

func TruncateMessageWithAlertID(message string, stringLimit int, warnMessage string) string {
	if len(message) <= stringLimit {
		return message
	}

	alertIndex := strings.LastIndex(message, "Alert Id:")
	if alertIndex == -1 {
		return fmt.Sprintf("%s\n%s", warnMessage, message[:stringLimit-len(warnMessage)-1])
	}

	alertText := message[alertIndex:]

	const formatSpecialCharsLength = 9
	maxTextLength := stringLimit - len(warnMessage) - len(alertText) - formatSpecialCharsLength

	if maxTextLength > 0 && alertIndex > maxTextLength {
		return fmt.Sprintf("%s\n...\n\n*%s*\n%s", message[:maxTextLength], warnMessage, alertText)
	}

	return fmt.Sprintf("%s\n%s", warnMessage, alertText)
}
