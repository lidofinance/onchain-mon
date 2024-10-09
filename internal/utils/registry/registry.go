package registry

import (
	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/utils/registry/teams"
)

const FallBackTeam = `default`

var CodeOwners = map[string]string{
	"ethereum-huge-tx":   teams.Analytics,
	"ethereum-steth":     teams.Protocol,
	"l2-bridge-arbitrum": teams.Protocol,
	"l2-bridge-balance":  teams.Protocol,
	"l2-bridge-ethereum": teams.Protocol,
	"l2-optimism":        teams.Protocol,
	"l2-bridge-base":     teams.Protocol,
	"l2-bridge-zksync":   teams.Protocol,
	"l2-bridge-mantle":   teams.Protocol,
	"l2-bridge-linea":    teams.Protocol,
	"l2-bridge-scroll":   teams.Protocol,
	"ethereum-financial": teams.Protocol,

	"lido-on-polygon": teams.CSM,
	"phishing-detect": teams.CSM,
	"polygon":         teams.CSM,

	"arb-subgraph": teams.UI,

	"ethereum-governance": teams.Governance,
	"multisig-watcher":    teams.Governance,
	"voting-watcher":      teams.Governance,

	"ethereum-validators-set": teams.ValSet,
	"devops-test-bot":         teams.DevOps,
	`host.docker.internal`:    FallBackTeam,
}

type AlertMapping = map[string]bool
type FindingMapping = map[databus.Severity]bool

var OnChainErrors = FindingMapping{
	databus.SeverityUnknown: true,
}

var OnChainUpdates = FindingMapping{
	databus.SeverityInfo:   true,
	databus.SeverityLow:    true,
	databus.SeverityMedium: true,
}

var OnChainAlerts = FindingMapping{
	databus.SeverityHigh:     true,
	databus.SeverityCritical: true,
}

var FallBackAlerts = FindingMapping{
	databus.SeverityUnknown:  true,
	databus.SeverityInfo:     true,
	databus.SeverityLow:      true,
	databus.SeverityMedium:   true,
	databus.SeverityHigh:     true,
	databus.SeverityCritical: true,
}
