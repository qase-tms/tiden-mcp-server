package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestViaChildrenFields_RoundTrip(t *testing.T) {
	var fact GateRequirementFact
	if err := json.Unmarshal([]byte(`{"requirementId":"r1","state":"no_test","coverageExemption":"child_specs","viaChildren":true,"childSpecs":3,"childSpecsCovered":1}`), &fact); err != nil {
		t.Fatal(err)
	}
	if fact.CoverageExemption != "child_specs" || !fact.ViaChildren || fact.ChildSpecs != 3 || fact.ChildSpecsCovered != 1 {
		t.Fatalf("gate fact: %+v", fact)
	}
	var sum SummaryRequirement
	if err := json.Unmarshal([]byte(`{"requirementId":"r1","display":"A-1","coverage":"verified","viaChildren":true,"childSpecs":2,"childSpecsCovered":2}`), &sum); err != nil {
		t.Fatal(err)
	}
	if !sum.ViaChildren || sum.ChildSpecs != 2 || sum.ChildSpecsCovered != 2 {
		t.Fatalf("summary row: %+v", sum)
	}
	var mr MatrixRequirement
	if err := json.Unmarshal([]byte(`{"requirementId":"r1","display":"A-1","coverage":"verified","cells":[],"viaChildren":true,"childSpecs":2,"childSpecsCovered":2}`), &mr); err != nil {
		t.Fatal(err)
	}
	if !mr.ViaChildren || mr.ChildSpecs != 2 {
		t.Fatalf("matrix row: %+v", mr)
	}
	var item LookupItemResult
	if err := json.Unmarshal([]byte(`{"id":"P1","coverageGapIds":["b"],"viaChildrenIds":["b"]}`), &item); err != nil {
		t.Fatal(err)
	}
	if len(item.ViaChildrenIDs) != 1 || item.ViaChildrenIDs[0] != "b" {
		t.Fatalf("lookup item: %+v", item)
	}
	out, _ := json.Marshal(sum)
	for _, want := range []string{`"viaChildren":true`, `"childSpecs":2`, `"childSpecsCovered":2`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("re-encoded row lacks %s: %s", want, out)
		}
	}
}

// A server that predates the fields sends none; the re-encoded rows gain none.
func TestViaChildrenFields_AbsentStaysAbsent(t *testing.T) {
	for _, v := range []any{
		GateRequirementFact{RequirementID: "r1", State: "no_test"},
		SummaryRequirement{RequirementID: "r1", Coverage: "no_test"},
		MatrixRequirement{RequirementID: "r1", Coverage: "no_test"},
		LookupItemResult{ID: "P1"},
	} {
		b, _ := json.Marshal(v)
		for _, key := range []string{"viaChildren", "childSpecs", "childSpecsCovered", "coverageExemption", "viaChildrenIds"} {
			if strings.Contains(string(b), key) {
				t.Errorf("%T gained %q: %s", v, key, b)
			}
		}
	}
}
