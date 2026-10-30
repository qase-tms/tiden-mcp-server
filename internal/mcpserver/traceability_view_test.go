package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

func rollupFixture(rows ...model.SummaryRequirement) model.TraceabilitySummary {
	return model.TraceabilitySummary{Components: []model.SummaryComponent{{ComponentID: "c1", Name: "api", Status: "ok", Requirements: rows}}}
}

// The rollup keeps the parent-spec mark on each spec it lists as not verified.
func TestRollup_KeepsViaChildrenMark(t *testing.T) {
	out := rollup(rollupFixture(
		model.SummaryRequirement{RequirementID: "r1", Display: "A-1", Coverage: "no_test", ViaChildren: true, ChildSpecs: 2, ChildSpecsCovered: 1},
		model.SummaryRequirement{RequirementID: "r2", Display: "A-2", Coverage: "no_test"},
	))
	got := out[0].NotVerified
	if len(got) != 2 || !got[0].ViaChildren || got[0].ChildSpecs != 2 || got[0].ChildSpecsCovered != 1 {
		t.Fatalf("rollup spec lost the mark: %+v", got)
	}
	if got[1].ViaChildren {
		t.Fatalf("plain spec gained the mark: %+v", got[1])
	}
}

// With no fields on any row (an older server) the rollup JSON has none either.
func TestRollup_NoViaChildrenFieldsUnchanged(t *testing.T) {
	out := rollup(rollupFixture(model.SummaryRequirement{RequirementID: "r1", Display: "A-1", Coverage: "no_test"}))
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"componentId":"c1","name":"api","status":"ok","specCount":1,"coverage":{"no_test":1},"notVerified":[{"display":"A-1","requirementId":"r1","coverage":"no_test"}]}]`
	if string(b) != want {
		t.Fatalf("rollup json changed:\ngot:  %s\nwant: %s", b, want)
	}
	if strings.Contains(string(b), "viaChildren") {
		t.Fatal("unexpected field")
	}
}

// A server that ignores view=summary answers with the full matrix; the summary
// built from it keeps the mark.
func TestSummaryFromMatrix_KeepsViaChildrenMark(t *testing.T) {
	m := model.TraceabilityMatrix{Components: []model.MatrixComponent{{ComponentID: "c1", Name: "api", Requirements: []model.MatrixRequirement{
		{RequirementID: "r1", Display: "A-1", Coverage: "verified", ViaChildren: true, ChildSpecs: 2, ChildSpecsCovered: 2, CoverageExemption: "child_specs"},
	}}}}
	got := summaryFromMatrix(m).Components[0].Requirements[0]
	if !got.ViaChildren || got.ChildSpecs != 2 || got.ChildSpecsCovered != 2 || got.CoverageExemption != "child_specs" {
		t.Fatalf("summary row lost the mark: %+v", got)
	}
}
