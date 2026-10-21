package attempts

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusInProgress    Status = "in_progress"
	StatusSubmitted     Status = "submitted"      // awaiting manual grading
	StatusAutoSubmitted Status = "auto_submitted" // ended by the server, awaiting manual grading
	StatusDisqualified  Status = "disqualified"
	StatusAbandoned     Status = "abandoned" // expired without auto-submit; not graded
	StatusEvaluated     Status = "evaluated" // fully graded
)

// EndReason records why an attempt stopped running.
type EndReason string

const (
	EndSubmitted      EndReason = "submitted"
	EndTimeExpired    EndReason = "time_expired"
	EndDisconnected   EndReason = "disconnected"
	EndViolationLimit EndReason = "violation_limit"
	EndExamClosed     EndReason = "exam_closed"
)

type SectionStatus string

const (
	SectionNotStarted SectionStatus = "locked" // never entered; availability is computed
	SectionInProgress SectionStatus = "in_progress"
	SectionCompleted  SectionStatus = "completed"
	SectionSkipped    SectionStatus = "skipped" // never entered before the attempt ended
)

// Attempt is one run of an exam by a candidate. Every timing input is frozen
// when the attempt starts.
type Attempt struct {
	ID             uuid.UUID
	ExamID         uuid.UUID
	UserID         uuid.UUID
	AssignmentID   *uuid.UUID
	AttemptNumber  int
	Status         Status
	DeviceID       *string
	Seed           int64
	StartedAt      time.Time
	TimerOriginAt  time.Time
	LimitSeconds   int
	WindowEndAt    *time.Time
	TimeMultiplier float64

	CurrentSectionID *uuid.UUID
	PausedAt         *time.Time
	PauseCount       int
	PausedSeconds    int
	LastHeartbeatAt  *time.Time
	DeadlineAt       *time.Time

	SubmittedAt *time.Time
	EndedReason *EndReason
	TotalScore  *float64
	MaxScore    *float64
	Passed      *bool
	ClientIP    *string
}

func (a *Attempt) running() bool { return a.Status == StatusInProgress }

// SectionState is a candidate's progress through one section.
type SectionState struct {
	SectionID uuid.UUID
	// Position is the section's place in this candidate's order.
	Position         int
	Status           SectionStatus
	TimeLimitSeconds *int
	TimeSpentSeconds int
	// EnteredAt is the start of the current visit; nil unless the section is
	// the open one and the attempt is not paused.
	EnteredAt   *time.Time
	UnlockedAt  *time.Time
	CompletedAt *time.Time
	// Cursor is the position of the question the candidate is on.
	Cursor       int
	ScoreAwarded *float64
	MaxScore     *float64
}

// PaperQuestion is one question served to an attempt.
type PaperQuestion struct {
	QuestionID uuid.UUID
	SectionID  uuid.UUID
	Position   int
	// OptionOrder is the order options are shown in; empty for questions
	// without options.
	OptionOrder   []uuid.UUID
	Points        float64
	FirstViewedAt *time.Time
}

// Response is a candidate's saved answer. Exactly one answer field is set.
type Response struct {
	QuestionID        uuid.UUID
	SelectedOptionID  *uuid.UUID
	SelectedOptionIDs []uuid.UUID
	TextResponse      *string
	BooleanResponse   *bool
	Flagged           bool
	UpdatedAt         time.Time
}

// answered reports whether the response holds an answer, as opposed to only
// a review flag.
func (r *Response) answered() bool {
	return r != nil && (r.SelectedOptionID != nil || len(r.SelectedOptionIDs) > 0 || r.TextResponse != nil || r.BooleanResponse != nil)
}

// Session authenticates the desktop app for one exam on one device.
type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExamID    uuid.UUID
	AttemptID *uuid.UUID
	DeviceID  string
	ExpiresAt time.Time
}
