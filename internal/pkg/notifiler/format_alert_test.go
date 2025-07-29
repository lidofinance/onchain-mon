package notifiler_test

import (
	"strings"
	"testing"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
	"github.com/lidofinance/onchain-mon/internal/utils/pointers"
)

func createBaseAlert() *databus.FindingDtoJson {
	return &databus.FindingDtoJson{
		Name:           "Test Alert",
		Description:    "Test alert description",
		Severity:       databus.SeverityHigh,
		AlertId:        "TEST-ALERT-ID",
		BlockTimestamp: pointers.IntPtr(1727965236),
		BlockNumber:    pointers.IntPtr(20884540),
		TxHash: pointers.StringPtr(
			"0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf",
		),
		BotName: "TestBot",
		Team:    "Protocol",
	}
}

func assertContains(t *testing.T, result, expectedPattern, testContext string) {
	t.Helper()
	if !strings.Contains(result, expectedPattern) {
		t.Errorf(
			"%s: result missing expected pattern.\nExpected pattern: %s\nActual result: %s",
			testContext,
			expectedPattern,
			result,
		)
	}
}

func assertNotContains(t *testing.T, result, unexpectedPattern, testContext string) {
	t.Helper()
	if strings.Contains(result, unexpectedPattern) {
		t.Errorf(
			"%s: result contains unexpected pattern.\nUnexpected pattern: %s\nActual result: %s",
			testContext,
			unexpectedPattern,
			result,
		)
	}
}

func TestFormatAlert_AlertLink_Nil(t *testing.T) {
	alert := createBaseAlert()
	alert.AlertLink = nil

	result := notifiler.FormatAlert(alert, "test-source", "etherscan.io")

	assertContains(t, result, "TEST-ALERT-ID", "AlertLink is nil")

	assertNotContains(t, result, "[TEST-ALERT-ID](", "AlertLink is nil")
}

func TestFormatAlert_AlertLink_Empty(t *testing.T) {
	alert := createBaseAlert()
	alert.AlertLink = pointers.StringPtr("")

	result := notifiler.FormatAlert(alert, "test-source", "etherscan.io")

	assertContains(t, result, "TEST-ALERT-ID", "AlertLink is empty")

	assertNotContains(t, result, "[TEST-ALERT-ID](", "AlertLink is empty")
}

func TestFormatAlert_AlertLink_Valid(t *testing.T) {
	githubURL := "https://github.com/search?q=TEST-ALERT-ID"
	alert := createBaseAlert()
	alert.AlertLink = pointers.StringPtr(githubURL)

	result := notifiler.FormatAlert(alert, "test-source", "etherscan.io")

	expectedPattern := "[TEST-ALERT-ID](" + githubURL + ")"
	assertContains(t, result, expectedPattern, "AlertLink is valid URL")

	assertContains(t, result, "[TEST-ALERT-ID](", "AlertLink is valid URL")
}
