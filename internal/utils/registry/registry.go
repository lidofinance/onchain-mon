package registry

import (
	"slices"
	"strings"

	"github.com/lidofinance/onchain-mon/generated/databus"
)

type FindingMapping = map[databus.Severity]bool
type FindingFilterMap = map[string]bool

type NotificationChannel string

const (
	Telegram NotificationChannel = `Telegram`
	Discord  NotificationChannel = `Discord`
	OpsGenie NotificationChannel = `OpsGenie`
	Slack    NotificationChannel = `Slack`
)

var CanonicalSeverities = FindingMapping{
	databus.SeverityUnknown:  true,
	databus.SeverityInfo:     true,
	databus.SeverityLow:      true,
	databus.SeverityMedium:   true,
	databus.SeverityHigh:     true,
	databus.SeverityCritical: true,
}

func CanonicalSeverityList() string {
	out := make([]string, 0, len(CanonicalSeverities))
	for severity := range CanonicalSeverities {
		out = append(out, string(severity))
	}
	slices.Sort(out)

	return strings.Join(out, ", ")
}
