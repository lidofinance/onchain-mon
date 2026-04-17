package notifiler_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
)

func newTestMetrics(t *testing.T) *metrics.Store {
	t.Helper()
	reg := prometheus.NewRegistry()
	return metrics.New(reg, "test", "test", "test")
}

func TestSendFinding_SkipsLowSeverity(t *testing.T) {
	m := newTestMetrics(t)
	og := notifiler.NewOpsgenie("key", nil, m, "local", "etherscan.io", "test")

	for _, sev := range []databus.Severity{databus.SeverityInfo, databus.SeverityLow, databus.SeverityMedium, databus.SeverityUnknown} {
		alert := &databus.FindingDtoJson{
			Name:        "test",
			Description: "desc",
			Severity:    sev,
			AlertId:     "TEST",
			BotName:     "bot",
			Team:        "team",
			UniqueKey:   "key",
		}
		// SendFinding should return nil without making any HTTP call for non-High/Critical severities.
		// httpClient is nil, so if it tried to send, it would panic.
		err := og.SendFinding(context.Background(), alert)
		if err != nil {
			t.Fatalf("SendFinding(%s) unexpected error: %v", sev, err)
		}
	}
}

func TestAlertPayload_DetailsContainsForwarderAttributes(t *testing.T) {
	payload := notifiler.AlertPayload{
		Message:  "test alert",
		Priority: "P1",
		Alias:    "mainnet-TEST",
		Details: map[string]string{
			"env":     "mainnet",
			"source":  "cluster1",
			"team":    "vroom",
			"botName": "unusual-activity",
			"alertId": "UNUSUAL-ACTIVITY-LOW-BALANCE",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	details, ok := decoded["details"].(map[string]interface{})
	if !ok {
		t.Fatal("details field missing or wrong type in serialized payload")
	}

	expected := map[string]string{
		"env":     "mainnet",
		"source":  "cluster1",
		"team":    "vroom",
		"botName": "unusual-activity",
		"alertId": "UNUSUAL-ACTIVITY-LOW-BALANCE",
	}
	for k, want := range expected {
		if got := details[k]; got != want {
			t.Errorf("details[%s] = %v, want %s", k, got, want)
		}
	}
}

func TestAlertPayload_NilDetailsOmitted(t *testing.T) {
	payload := notifiler.AlertPayload{
		Message:  "test alert",
		Priority: "P2",
		Alias:    "test-TEST",
		Details:  nil,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := decoded["details"]; ok {
		t.Error("details field should be omitted when nil")
	}
}
