package mcpserver

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/qase-tms/tiden-mcp-server/internal/model"
)

// maxNotVerifiedPerComponent caps the list of unverified specs per component
// in the rollup; the rest is reported as a count.
const maxNotVerifiedPerComponent = 50

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// validateSubjectID rejects anything but a spec or component UUID. A display
// reference such as QA-12 is not accepted by the server filter, which would
// silently return the whole product.
func validateSubjectID(field, value string) error {
	if value == "" || uuidPattern.MatchString(value) {
		return nil
	}
	if field == "feature_id" {
		return fmt.Errorf("%s must be the spec's UUID, not a display reference such as QA-12; get the id from get_spec or list_specs (the id field of the spec)", field)
	}
	return fmt.Errorf("%s must be a component UUID (the componentId field of an earlier get_traceability rollup)", field)
}

// ---- normalisation ---------------------------------------------------------

// summaryFromMatrix drops the per-test cells of a full matrix. It serves a
// server that ignored view=summary and answered with the complete matrix.
func summaryFromMatrix(m model.TraceabilityMatrix) model.TraceabilitySummary {
	out := model.TraceabilitySummary{Components: make([]model.SummaryComponent, 0, len(m.Components))}
	for _, c := range m.Components {
		sc := model.SummaryComponent{
			ComponentID: c.ComponentID, Name: c.Name, Status: c.Status,
			ResidualRisk: c.ResidualRisk, Ceiling: c.Ceiling, Repository: c.Repository,
			Requirements: make([]model.SummaryRequirement, 0, len(c.Requirements)),
		}
		for _, r := range c.Requirements {
			sc.Requirements = append(sc.Requirements, model.SummaryRequirement{
				RequirementID: r.RequirementID, Display: r.Display, Title: r.Title,
				ParentID: r.ParentID, BranchStatus: r.BranchStatus, Coverage: r.Coverage,
				CoverageExemption: r.CoverageExemption, ViaChildren: r.ViaChildren,
				ChildSpecs: r.ChildSpecs, ChildSpecsCovered: r.ChildSpecsCovered,
			})
		}
		out.Components = append(out.Components, sc)
	}
	return out
}

// ---- subtree filter --------------------------------------------------------

// subtreeIDs returns the requirementIds of every row that is the subject or a
// descendant of it. Rows are indexed by requirementId and canonicalId because
// a branch row carries its branch-local id while parentId names main ids.
func subtreeIDs(subject string, rows []model.SummaryRequirement) map[string]bool {
	byID := map[string]model.SummaryRequirement{}
	for _, r := range rows {
		byID[r.RequirementID] = r
		if r.CanonicalID != "" {
			byID[r.CanonicalID] = r
		}
	}
	keep := map[string]bool{}
	for _, r := range rows {
		cur, depth := r, 0
		for depth < 1000 {
			if cur.RequirementID == subject || cur.CanonicalID == subject {
				keep[r.RequirementID] = true
				break
			}
			parent, ok := byID[cur.ParentID]
			if cur.ParentID == "" || !ok {
				if cur.ParentID == subject {
					keep[r.RequirementID] = true
				}
				break
			}
			cur = parent
			depth++
		}
	}
	return keep
}

func allSummaryRows(comps []model.SummaryComponent) []model.SummaryRequirement {
	var rows []model.SummaryRequirement
	for _, c := range comps {
		rows = append(rows, c.Requirements...)
	}
	return rows
}

// filterSummaryToFeature keeps the subject's subtree and drops components left
// empty. dropped reports whether any row or component was removed, which means
// the server did not apply the filter.
func filterSummaryToFeature(s model.TraceabilitySummary, subject string) (model.TraceabilitySummary, bool) {
	keep := subtreeIDs(subject, allSummaryRows(s.Components))
	out := model.TraceabilitySummary{Components: []model.SummaryComponent{}}
	dropped := false
	for _, c := range s.Components {
		kept := make([]model.SummaryRequirement, 0, len(c.Requirements))
		for _, r := range c.Requirements {
			if keep[r.RequirementID] {
				kept = append(kept, r)
			} else {
				dropped = true
			}
		}
		if len(kept) == 0 {
			dropped = true
			continue
		}
		c.Requirements = kept
		out.Components = append(out.Components, c)
	}
	return out, dropped
}

func filterMatrixToFeature(m model.TraceabilityMatrix, subject string) (model.TraceabilityMatrix, bool) {
	var rows []model.SummaryRequirement
	for _, c := range m.Components {
		for _, r := range c.Requirements {
			rows = append(rows, model.SummaryRequirement{
				RequirementID: r.RequirementID, ParentID: r.ParentID,
			})
		}
	}
	keep := subtreeIDs(subject, rows)
	out := model.TraceabilityMatrix{Components: []model.MatrixComponent{}}
	dropped := false
	for _, c := range m.Components {
		kept := make([]model.MatrixRequirement, 0, len(c.Requirements))
		for _, r := range c.Requirements {
			if keep[r.RequirementID] {
				kept = append(kept, r)
			} else {
				dropped = true
			}
		}
		if len(kept) == 0 {
			dropped = true
			continue
		}
		c.Requirements = kept
		out.Components = append(out.Components, c)
	}
	return out, dropped
}

func filterSummaryToComponent(s model.TraceabilitySummary, id string) (model.TraceabilitySummary, bool) {
	out := model.TraceabilitySummary{Components: []model.SummaryComponent{}}
	dropped := false
	for _, c := range s.Components {
		if c.ComponentID == id {
			out.Components = append(out.Components, c)
		} else {
			dropped = true
		}
	}
	return out, dropped
}

func filterMatrixToComponent(m model.TraceabilityMatrix, id string) (model.TraceabilityMatrix, bool) {
	out := model.TraceabilityMatrix{Components: []model.MatrixComponent{}}
	dropped := false
	for _, c := range m.Components {
		if c.ComponentID == id {
			out.Components = append(out.Components, c)
		} else {
			dropped = true
		}
	}
	return out, dropped
}

// ---- rollup ----------------------------------------------------------------

type rollupSpec struct {
	Display       string `json:"display"`
	RequirementID string `json:"requirementId"`
	Coverage      string `json:"coverage"`
	// The parent-spec mark, copied from the summary row; absent otherwise.
	ViaChildren       bool `json:"viaChildren,omitempty"`
	ChildSpecs        int  `json:"childSpecs,omitempty"`
	ChildSpecsCovered int  `json:"childSpecsCovered,omitempty"`
}

type rollupComponent struct {
	ComponentID string         `json:"componentId"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	SpecCount   int            `json:"specCount"`
	Coverage    map[string]int `json:"coverage"`
	NotVerified []rollupSpec   `json:"notVerified"`
	// NotVerifiedMore counts unverified specs beyond the listed cap.
	NotVerifiedMore int `json:"notVerifiedMore,omitempty"`
}

func rollup(s model.TraceabilitySummary) []rollupComponent {
	out := make([]rollupComponent, 0, len(s.Components))
	for _, c := range s.Components {
		rc := rollupComponent{
			ComponentID: c.ComponentID, Name: c.Name, Status: c.Status,
			SpecCount: len(c.Requirements), Coverage: map[string]int{}, NotVerified: []rollupSpec{},
		}
		for _, r := range c.Requirements {
			rc.Coverage[r.Coverage]++
			if r.Coverage == "verified" {
				continue
			}
			if len(rc.NotVerified) < maxNotVerifiedPerComponent {
				rc.NotVerified = append(rc.NotVerified, rollupSpec{
					Display: r.Display, RequirementID: r.RequirementID, Coverage: r.Coverage,
					ViaChildren: r.ViaChildren, ChildSpecs: r.ChildSpecs, ChildSpecsCovered: r.ChildSpecsCovered,
				})
			} else {
				rc.NotVerifiedMore++
			}
		}
		out = append(out, rc)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ---- output ----------------------------------------------------------------

const clientFilterNote = "The server did not apply the filter, so the rows were filtered by this tool. Component status is component-wide, not computed over the filtered rows only."

// traceabilityOutput is the get_traceability result. Components holds
// rollupComponent, model.SummaryComponent or model.MatrixComponent values,
// depending on View.
type traceabilityOutput struct {
	View            string `json:"view"`
	FeatureFilter   string `json:"featureFilter,omitempty"`
	ComponentFilter string `json:"componentFilter,omitempty"`
	Note            string `json:"note,omitempty"`
	Components      any    `json:"components"`
}
