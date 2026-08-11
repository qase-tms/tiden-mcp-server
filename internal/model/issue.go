package model

// Issue is a fingerprint-grouped error. timesSeen is int64 over protojson and
// therefore arrives as a JSON string.
type Issue struct {
	ID             string  `json:"id"`
	ProductID      string  `json:"productId"`
	Title          string  `json:"title"`
	Culprit        string  `json:"culprit"`
	Level          string  `json:"level"`
	Platform       string  `json:"platform"`
	Status         string  `json:"status"`
	TimesSeen      int64   `json:"timesSeen,string"`
	FirstSeen      string  `json:"firstSeen,omitempty"`
	LastSeen       string  `json:"lastSeen,omitempty"`
	FirstReleaseID *string `json:"firstReleaseId,omitempty"`
	LastReleaseID  *string `json:"lastReleaseId,omitempty"`
	ComponentID    *string `json:"componentId,omitempty"`
	ResolvedAt     *string `json:"resolvedAt,omitempty"`
	RegressedAt    *string `json:"regressedAt,omitempty"`
}

// Frame is one server-resolved stack frame. State is
// resolved|raw|map_missing|map_mismatch|degraded.
type Frame struct {
	Function    string   `json:"function,omitempty"`
	AbsPath     string   `json:"absPath,omitempty"`
	Filename    string   `json:"filename,omitempty"`
	Lineno      int      `json:"lineno,omitempty"`
	Colno       int      `json:"colno,omitempty"`
	InApp       bool     `json:"inApp,omitempty"`
	State       string   `json:"state,omitempty"`
	ContextLine string   `json:"contextLine,omitempty"`
	PreContext  []string `json:"preContext,omitempty"`
	PostContext []string `json:"postContext,omitempty"`
}

// IssueEvent is one occurrence. Payload is the full raw event JSON; the issue
// tools clear it unless include_payload is set, because it would swamp an
// agent's context.
type IssueEvent struct {
	ID              string  `json:"id"`
	EventID         string  `json:"eventId"`
	Level           string  `json:"level"`
	Message         string  `json:"message"`
	ExceptionType   string  `json:"exceptionType"`
	ExceptionValue  string  `json:"exceptionValue"`
	Platform        string  `json:"platform"`
	ReleaseID       *string `json:"releaseId,omitempty"`
	EnvironmentID   *string `json:"environmentId,omitempty"`
	ReleaseName     string  `json:"releaseName,omitempty"`
	EnvironmentName string  `json:"environmentName,omitempty"`
	Payload         string  `json:"payload,omitempty"`
	ReceivedAt      string  `json:"receivedAt,omitempty"`
	Frames          []Frame `json:"frames,omitempty"`
}

type EventBucket struct {
	Start string `json:"start,omitempty"`
	Count int    `json:"count,omitempty"`
}

type IssueEnvironmentCount struct {
	EnvironmentID   *string `json:"environmentId,omitempty"`
	EnvironmentName string  `json:"environmentName,omitempty"`
	Count           int     `json:"count,omitempty"`
}

type IssueEventStats struct {
	Interval     string                  `json:"interval,omitempty"`
	Buckets      []EventBucket           `json:"buckets,omitempty"`
	Last24h      int                     `json:"last24h,omitempty"`
	Environments []IssueEnvironmentCount `json:"environments,omitempty"`
}

type CoveringTest struct {
	ID         string `json:"id"`
	SeqNum     int    `json:"seqNum,omitempty"`
	Title      string `json:"title,omitempty"`
	LastStatus string `json:"lastStatus,omitempty"`
}

type SuspectRequirement struct {
	ID           string         `json:"id"`
	SeqNum       int            `json:"seqNum,omitempty"`
	Title        string         `json:"title,omitempty"`
	MatchedPaths []string       `json:"matchedPaths,omitempty"`
	Coverage     string         `json:"coverage,omitempty"`
	Tests        []CoveringTest `json:"tests,omitempty"`
}

// IssueFixContext is the composed triage pack: what broke, where, which files
// to open, and whether a test already covers them.
type IssueFixContext struct {
	Issue               *Issue                  `json:"issue,omitempty"`
	LatestEvent         *IssueEvent             `json:"latestEvent,omitempty"`
	SuspectPaths        []string                `json:"suspectPaths,omitempty"`
	Environments        []IssueEnvironmentCount `json:"environments,omitempty"`
	FirstReleaseName    string                  `json:"firstReleaseName,omitempty"`
	LastReleaseName     string                  `json:"lastReleaseName,omitempty"`
	Regressed           bool                    `json:"regressed,omitempty"`
	Component           *Component              `json:"component,omitempty"`
	SuspectRequirements []SuspectRequirement    `json:"suspectRequirements,omitempty"`
	TruncationSignals   []string                `json:"truncationSignals,omitempty"`
}
