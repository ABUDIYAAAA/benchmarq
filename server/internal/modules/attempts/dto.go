package attempts

import (
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/google/uuid"
)

// Candidate-facing payloads. Nothing here may carry an answer key,
// explanation, rubric or sample answer: questions are converted field by
// field from the authoring model instead of being embedded.

// HeartbeatIntervalSeconds is how often the desktop app should check in.
const HeartbeatIntervalSeconds = 15

// ---------------------------------------------------------------------------
// Web portal
// ---------------------------------------------------------------------------

// CandidateExam is one exam on the candidate's dashboard.
type CandidateExam struct {
	ExamID       uuid.UUID    `json:"exam_id"`
	OrgID        uuid.UUID    `json:"org_id"`
	OrgName      string       `json:"org_name"`
	Title        string       `json:"title"`
	Description  *string      `json:"description,omitempty"`
	Availability Availability `json:"availability"`
}

// LaunchGrant is a single-use code the portal hands to the desktop app.
type LaunchGrant struct {
	Code      string    `json:"code"`
	LaunchURL string    `json:"launch_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ---------------------------------------------------------------------------
// Desktop app
// ---------------------------------------------------------------------------

type OpenSessionRequest struct {
	LaunchCode string `json:"launch_code" validate:"required,max=128"`
	DeviceID   string `json:"device_id" validate:"required,min=16,max=128"`
}

// SessionGrant authenticates the desktop app. The token is sent as a bearer
// token together with the device ID header on every later request.
type SessionGrant struct {
	Token     string        `json:"token"`
	ExpiresAt time.Time     `json:"expires_at"`
	State     *RuntimeState `json:"state"`
}

// RuntimeState is everything the desktop app renders outside a question.
type RuntimeState struct {
	ServerTime time.Time      `json:"server_time"`
	Exam       ExamBrief      `json:"exam"`
	Rules      CandidateRules `json:"rules"`
	// Availability is set before the attempt starts.
	Availability *Availability `json:"availability,omitempty"`
	Attempt      *AttemptView  `json:"attempt,omitempty"`
}

type ExamBrief struct {
	ID              uuid.UUID      `json:"id"`
	Title           string         `json:"title"`
	Description     *string        `json:"description,omitempty"`
	Instructions    *string        `json:"instructions,omitempty"`
	DurationMinutes int            `json:"duration_minutes"`
	Sections        []SectionBrief `json:"sections"`
}

type SectionBrief struct {
	Name             string            `json:"name"`
	Type             exams.SectionType `json:"type"`
	QuestionCount    int               `json:"question_count"`
	TimeLimitSeconds *int              `json:"time_limit_seconds,omitempty"`
}

// CandidateRules are the configured behaviours the desktop app must enforce
// or display. The server enforces everything it can observe itself.
type CandidateRules struct {
	SectionProgressionMode   exams.SectionProgressionMode `json:"section_progression_mode"`
	QuestionNavigationMode   exams.QuestionNavigationMode `json:"question_navigation_mode"`
	AllowRevisitingQuestions bool                         `json:"allow_revisiting_questions"`
	AllowBookmarkForReview   bool                         `json:"allow_bookmark_for_review"`
	AutoAdvanceOnSelection   bool                         `json:"auto_advance_on_selection"`

	AllowPause                     bool `json:"allow_pause"`
	MaxPauseCount                  int  `json:"max_pause_count"`
	MaxTotalPauseMinutes           int  `json:"max_total_pause_minutes"`
	MinTimeBeforeSubmissionMinutes int  `json:"min_time_before_submission_minutes"`
	AllowResumeOnDisconnect        bool `json:"allow_resume_on_disconnect"`
	ReconnectionGraceSeconds       int  `json:"reconnection_grace_seconds"`
	HeartbeatIntervalSeconds       int  `json:"heartbeat_interval_seconds"`

	Experience exams.ExperienceConfig `json:"experience"`
	Proctoring ProctoringRules        `json:"proctoring"`
}

type ProctoringRules struct {
	Mode                          exams.ProctoringMode `json:"mode"`
	EnforceFullscreen             bool                 `json:"enforce_fullscreen"`
	MaxFullscreenExitCount        int                  `json:"max_fullscreen_exit_count"`
	TrackTabFocus                 bool                 `json:"track_tab_focus"`
	MaxTabSwitchCount             int                  `json:"max_tab_switch_count"`
	TabSwitchGraceSeconds         int                  `json:"tab_switch_grace_seconds"`
	BlockCopyPasteClipboard       bool                 `json:"block_copy_paste_clipboard"`
	BlockDeveloperToolsKeys       bool                 `json:"block_developer_tools_keys"`
	WebcamSnapshotsEnabled        bool                 `json:"webcam_snapshots_enabled"`
	WebcamSnapshotIntervalSeconds int                  `json:"webcam_snapshot_interval_seconds"`
	AudioMonitoringEnabled        bool                 `json:"audio_monitoring_enabled"`
}

func rulesFrom(cfg *exams.ExamConfig) CandidateRules {
	nav, sc, pr := cfg.Navigation, cfg.Schedule, cfg.Proctoring
	return CandidateRules{
		SectionProgressionMode:   nav.SectionProgressionMode,
		QuestionNavigationMode:   nav.QuestionNavigationMode,
		AllowRevisitingQuestions: nav.AllowRevisitingQuestions && nav.QuestionNavigationMode != exams.NavigationSequentialOnly,
		AllowBookmarkForReview:   nav.AllowBookmarkForReview,
		AutoAdvanceOnSelection:   nav.AutoAdvanceOnSelection,

		AllowPause:                     sc.AllowCandidatePause,
		MaxPauseCount:                  sc.MaxPauseCount,
		MaxTotalPauseMinutes:           sc.MaxTotalPauseMinutes,
		MinTimeBeforeSubmissionMinutes: sc.MinTimeBeforeSubmissionMinutes,
		AllowResumeOnDisconnect:        cfg.Attempts.AllowResumeOnDisconnect,
		ReconnectionGraceSeconds:       sc.ReconnectionGraceSeconds,
		HeartbeatIntervalSeconds:       HeartbeatIntervalSeconds,

		Experience: cfg.Experience,
		Proctoring: ProctoringRules{
			Mode:                          pr.ProctoringMode,
			EnforceFullscreen:             pr.EnforceFullscreen,
			MaxFullscreenExitCount:        pr.MaxFullscreenExitCount,
			TrackTabFocus:                 pr.TrackTabFocus,
			MaxTabSwitchCount:             pr.MaxTabSwitchCount,
			TabSwitchGraceSeconds:         pr.TabSwitchGraceSeconds,
			BlockCopyPasteClipboard:       pr.BlockCopyPasteClipboard,
			BlockDeveloperToolsKeys:       pr.BlockDeveloperToolsKeys,
			WebcamSnapshotsEnabled:        pr.WebcamSnapshotsEnabled,
			WebcamSnapshotIntervalSeconds: pr.WebcamSnapshotIntervalSeconds,
			AudioMonitoringEnabled:        pr.AudioMonitoringEnabled,
		},
	}
}

type AttemptView struct {
	ID            uuid.UUID        `json:"id"`
	AttemptNumber int              `json:"attempt_number"`
	Status        Status           `json:"status"`
	StartedAt     time.Time        `json:"started_at"`
	SubmittedAt   *time.Time       `json:"submitted_at,omitempty"`
	EndedReason   *EndReason       `json:"ended_reason,omitempty"`
	Clock         *ClockView       `json:"clock,omitempty"`
	Sections      []SectionView    `json:"sections"`
	Current       *OpenSectionView `json:"current_section,omitempty"`
	Violations    map[string]int   `json:"violations"`
	Answered      int              `json:"answered"`
	Total         int              `json:"total"`
}

// ClockView is the authoritative timer state. Clients should render
// countdowns from deadline_at and server_time, not from local clocks.
type ClockView struct {
	ServerTime       time.Time  `json:"server_time"`
	DeadlineAt       time.Time  `json:"deadline_at"`
	RemainingSeconds int        `json:"remaining_seconds"`
	Paused           bool       `json:"paused"`
	PausesLeft       int        `json:"pauses_left"`
	PauseSecondsLeft int        `json:"pause_seconds_left"`
	CanSubmitAt      time.Time  `json:"can_submit_at"`
	Section          *TimerView `json:"section,omitempty"`
}

type TimerView struct {
	SectionID        uuid.UUID `json:"section_id"`
	DeadlineAt       time.Time `json:"deadline_at"`
	RemainingSeconds int       `json:"remaining_seconds"`
}

type SectionView struct {
	ID               uuid.UUID         `json:"id"`
	Name             string            `json:"name"`
	Type             exams.SectionType `json:"type"`
	Instructions     *string           `json:"instructions,omitempty"`
	Position         int               `json:"position"`
	Status           SectionStatus     `json:"status"`
	IsOpen           bool              `json:"is_open"`
	CanEnter         bool              `json:"can_enter"`
	LockReason       string            `json:"lock_reason,omitempty"`
	UnlocksAt        *time.Time        `json:"unlocks_at,omitempty"`
	QuestionCount    int               `json:"question_count"`
	AnsweredCount    int               `json:"answered_count"`
	TimeLimitSeconds *int              `json:"time_limit_seconds,omitempty"`
	RemainingSeconds *int              `json:"remaining_seconds,omitempty"`
}

// OpenSectionView describes the open section's question slots. It carries
// no question content.
type OpenSectionView struct {
	SectionID uuid.UUID  `json:"section_id"`
	Cursor    int        `json:"cursor"`
	Slots     []SlotView `json:"slots"`
}

type SlotView struct {
	Position  int  `json:"position"`
	Viewed    bool `json:"viewed"`
	Answered  bool `json:"answered"`
	Flagged   bool `json:"flagged"`
	Reachable bool `json:"reachable"`
}

type QuestionView struct {
	SectionID  uuid.UUID `json:"section_id"`
	Position   int       `json:"position"`
	IsLast     bool      `json:"is_last"`
	QuestionID uuid.UUID `json:"question_id"`
	Type       string    `json:"type"`
	Prompt     string    `json:"prompt"`
	Points     float64   `json:"points"`
	// Penalty is the marks lost for a wrong answer, when negative marking applies.
	Penalty     *float64         `json:"penalty,omitempty"`
	Options     []OptionView     `json:"options,omitempty"`
	MultiSelect *MultiSelectView `json:"multi_select,omitempty"`
	ShortAnswer *TextLimitsView  `json:"short_answer,omitempty"`
	Essay       *EssayView       `json:"essay,omitempty"`
	Response    *ResponseView    `json:"response,omitempty"`
	Clock       *ClockView       `json:"clock"`
}

type OptionView struct {
	ID      uuid.UUID `json:"id"`
	Content string    `json:"content"`
}

type MultiSelectView struct {
	MinChoices int  `json:"min_choices"`
	MaxChoices *int `json:"max_choices,omitempty"`
}

type TextLimitsView struct {
	MaxLength *int `json:"max_length,omitempty"`
	MaxWords  *int `json:"max_words,omitempty"`
}

type EssayView struct {
	MinWords      *int `json:"min_words,omitempty"`
	MaxWords      *int `json:"max_words,omitempty"`
	MaxLength     *int `json:"max_length,omitempty"`
	AllowRichText bool `json:"allow_rich_text"`
}

type ResponseView struct {
	SelectedOptionID  *uuid.UUID  `json:"selected_option_id,omitempty"`
	SelectedOptionIDs []uuid.UUID `json:"selected_option_ids,omitempty"`
	Text              *string     `json:"text,omitempty"`
	Boolean           *bool       `json:"boolean,omitempty"`
	Flagged           bool        `json:"flagged"`
	SavedAt           time.Time   `json:"saved_at"`
}

type NavigateRequest struct {
	Position int `json:"position" validate:"gte=0"`
}

// SaveResponseRequest answers the question under the cursor. Send the field
// matching the question type, clear to remove the answer, or only flagged
// to toggle the review flag.
type SaveResponseRequest struct {
	QuestionID        uuid.UUID   `json:"question_id" validate:"required"`
	SelectedOptionID  *uuid.UUID  `json:"selected_option_id"`
	SelectedOptionIDs []uuid.UUID `json:"selected_option_ids" validate:"omitempty,max=26,unique"`
	Text              *string     `json:"text" validate:"omitempty,max=500000"`
	Boolean           *bool       `json:"boolean"`
	Clear             bool        `json:"clear"`
	Flagged           *bool       `json:"flagged"`
}

// EventRequest reports something the desktop app observed.
type EventRequest struct {
	Type string `json:"type" validate:"required,oneof=fullscreen_exit tab_switch clipboard_action dev_tools_opened multiple_faces no_face audio_detected"`
	// DurationMs is how long focus was lost, for tab_switch.
	DurationMs *int           `json:"duration_ms" validate:"omitempty,gte=0,lte=86400000"`
	ClientTime *time.Time     `json:"client_time"`
	Details    map[string]any `json:"details" validate:"omitempty,max=20"`
}

type EventResult struct {
	Counted    bool           `json:"counted"`
	Violations map[string]int `json:"violations"`
	// Ended is true when this event exceeded a limit and ended the attempt.
	Ended bool `json:"ended"`
}

// ---------------------------------------------------------------------------
// Staff
// ---------------------------------------------------------------------------

type AttemptSummary struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	Email           string     `json:"email"`
	FirstName       string     `json:"first_name"`
	LastName        *string    `json:"last_name,omitempty"`
	AttemptNumber   int        `json:"attempt_number"`
	Status          Status     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	SubmittedAt     *time.Time `json:"submitted_at,omitempty"`
	EndedReason     *EndReason `json:"ended_reason,omitempty"`
	DeviceBound     bool       `json:"device_bound"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	DeadlineAt      *time.Time `json:"deadline_at,omitempty"`
	TotalScore      *float64   `json:"total_score,omitempty"`
	MaxScore        *float64   `json:"max_score,omitempty"`
}
