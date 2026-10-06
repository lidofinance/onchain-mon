package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func Test_canonical_severities_match_the_finding_schema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "brief", "databus", "finding.dto.json"))
	if err != nil {
		t.Fatalf("could not read the finding schema: %v", err)
	}

	var schema struct {
		Definitions struct {
			Severity struct {
				Enum []string `json:"enum"`
			} `json:"Severity"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("could not parse the finding schema: %v", err)
	}

	want := schema.Definitions.Severity.Enum
	if len(want) == 0 {
		t.Fatal("the finding schema has no Severity enum")
	}

	got := make([]string, 0, len(CanonicalSeverities))
	for severity := range CanonicalSeverities {
		got = append(got, string(severity))
	}
	slices.Sort(got)
	want = slices.Sorted(slices.Values(want))

	if !slices.Equal(got, want) {
		t.Errorf("CanonicalSeverities is %v, schema says %v", got, want)
	}
}
