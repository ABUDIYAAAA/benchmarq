package exams

import (
	"time"

	"github.com/ABUDIYAAAA/benchmarq/pkg/model"
	"github.com/google/uuid"
)

type Exam struct {
	model.Model
	OrgID           uuid.UUID  `json:"org_id" db:"org_id"`
	CreatedByUserID uuid.UUID  `json:"created_by_user_id" db:"created_by_user_id"`
	Title           string     `json:"title" db:"title"`
	Code            *string    `json:"code,omitempty" db:"code"`
	Description     *string    `json:"description,omitempty" db:"description"`
	Instructions    *string    `json:"instructions,omitempty" db:"instructions"`
	Status          ExamStatus `json:"status" db:"status"`
	TotalMarks      float64    `json:"total_marks" db:"total_marks"`
	PassingMarks    *float64   `json:"passing_marks,omitempty" db:"passing_marks"`
	PublishedAt     *time.Time `json:"published_at,omitempty" db:"published_at"`
}

// ExamConfig is the full rule set of an exam (one row of exam_configurations),
// grouped by category. The struct is also the merge target for PATCH
// requests, so validation tags live here.
type ExamConfig struct {
	ExamID uuid.UUID `json:"-" db:"exam_id"`

	Schedule      ScheduleConfig      `json:"schedule"`
	Navigation    NavigationConfig    `json:"navigation"`
	Randomization RandomizationConfig `json:"randomization"`
	Scoring       ScoringConfig       `json:"scoring"`
	Proctoring    ProctoringConfig    `json:"proctoring"`
	Experience    ExperienceConfig    `json:"experience"`
	Coding        CodingConfig        `json:"coding"`
	Results       ResultsConfig       `json:"results"`
	Attempts      AttemptPolicy       `json:"attempts"`

	// CustomSettings is an extension point for plugins. PATCH merges keys.
	CustomSettings map[string]any `json:"custom_settings" db:"custom_settings"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type ScheduleConfig struct {
	ScheduleType                   ScheduleType `json:"schedule_type" db:"schedule_type" validate:"required,oneof=fixed_window flexible_window open_ended"`
	StartTime                      *time.Time   `json:"start_time" db:"start_time"`
	EndTime                        *time.Time   `json:"end_time" db:"end_time"`
	TotalDurationMinutes           int          `json:"total_duration_minutes" db:"total_duration_minutes" validate:"gte=1,lte=1440"`
	PerSectionTiming               bool         `json:"per_section_timing" db:"per_section_timing"`
	PerQuestionTiming              bool         `json:"per_question_timing" db:"per_question_timing"`
	LateJoinGraceMinutes           int          `json:"late_join_grace_minutes" db:"late_join_grace_minutes" validate:"gte=0,lte=1440"`
	MinTimeBeforeSubmissionMinutes int          `json:"min_time_before_submission_minutes" db:"min_time_before_submission_minutes" validate:"gte=0,lte=1440"`
	AutoSubmitOnExpiry             bool         `json:"auto_submit_on_expiry" db:"auto_submit_on_expiry"`
	AllowCandidatePause            bool         `json:"allow_candidate_pause" db:"allow_candidate_pause"`
	MaxPauseCount                  int          `json:"max_pause_count" db:"max_pause_count" validate:"gte=0,lte=100"`
	MaxTotalPauseMinutes           int          `json:"max_total_pause_minutes" db:"max_total_pause_minutes" validate:"gte=0,lte=1440"`
	ReconnectionGraceSeconds       int          `json:"reconnection_grace_seconds" db:"reconnection_grace_seconds" validate:"gte=0,lte=3600"`
}

type NavigationConfig struct {
	SectionProgressionMode   SectionProgressionMode `json:"section_progression_mode" db:"section_progression_mode" validate:"required,oneof=linear_strict linear_relaxed free_navigation timed_sequential"`
	QuestionNavigationMode   QuestionNavigationMode `json:"question_navigation_mode" db:"question_navigation_mode" validate:"required,oneof=free_jump sequential_only mandatory_answer"`
	AllowRevisitingQuestions bool                   `json:"allow_revisiting_questions" db:"allow_revisiting_questions"`
	AllowBookmarkForReview   bool                   `json:"allow_bookmark_for_review" db:"allow_bookmark_for_review"`
	AutoAdvanceOnSelection   bool                   `json:"auto_advance_on_selection" db:"auto_advance_on_selection"`
}

type RandomizationConfig struct {
	ShuffleSections  bool `json:"shuffle_sections" db:"shuffle_sections"`
	ShuffleQuestions bool `json:"shuffle_questions" db:"shuffle_questions"`
	ShuffleOptions   bool `json:"shuffle_options" db:"shuffle_options"`
	// EnableQuestionPool makes PoolSampleCount the default number of questions
	// drawn per section. Sections can override it with question_pick_count.
	EnableQuestionPool    bool `json:"enable_question_pool" db:"enable_question_pool"`
	PoolSampleCount       *int `json:"pool_sample_count" db:"pool_sample_count" validate:"omitempty,gte=1,lte=1000"`
	DeterministicUserSeed bool `json:"deterministic_user_seed" db:"deterministic_user_seed"`
}

type ScoringConfig struct {
	NegativeMarkingEnabled        bool                   `json:"negative_marking_enabled" db:"negative_marking_enabled"`
	NegativeMarkingPenaltyPercent float64                `json:"negative_marking_penalty_percent" db:"negative_marking_penalty_percent" validate:"gte=0,lte=100"`
	MultiSelectScoringMode        MultiSelectScoringMode `json:"multi_select_scoring_mode" db:"multi_select_scoring_mode" validate:"required,oneof=all_or_nothing partial_with_penalty partial_no_penalty"`
	PassingPercentage             float64                `json:"passing_percentage" db:"passing_percentage" validate:"gte=0,lte=100"`
	GradingStrategy               string                 `json:"grading_strategy" db:"grading_strategy" validate:"required,oneof=automated manual hybrid"`
	BlindGradingEnabled           bool                   `json:"blind_grading_enabled" db:"blind_grading_enabled"`
}

type ProctoringConfig struct {
	ProctoringMode                ProctoringMode `json:"proctoring_mode" db:"proctoring_mode" validate:"required,oneof=disabled browser_only ai_proctored live_proctored"`
	EnforceFullscreen             bool           `json:"enforce_fullscreen" db:"enforce_fullscreen"`
	MaxFullscreenExitCount        int            `json:"max_fullscreen_exit_count" db:"max_fullscreen_exit_count" validate:"gte=0,lte=100"`
	TrackTabFocus                 bool           `json:"track_tab_focus" db:"track_tab_focus"`
	MaxTabSwitchCount             int            `json:"max_tab_switch_count" db:"max_tab_switch_count" validate:"gte=0,lte=100"`
	TabSwitchGraceSeconds         int            `json:"tab_switch_grace_seconds" db:"tab_switch_grace_seconds" validate:"gte=0,lte=300"`
	BlockCopyPasteClipboard       bool           `json:"block_copy_paste_clipboard" db:"block_copy_paste_clipboard"`
	BlockDeveloperToolsKeys       bool           `json:"block_developer_tools_keys" db:"block_developer_tools_keys"`
	WebcamSnapshotsEnabled        bool           `json:"webcam_snapshots_enabled" db:"webcam_snapshots_enabled"`
	WebcamSnapshotIntervalSeconds int            `json:"webcam_snapshot_interval_seconds" db:"webcam_snapshot_interval_seconds" validate:"gte=5,lte=3600"`
	AudioMonitoringEnabled        bool           `json:"audio_monitoring_enabled" db:"audio_monitoring_enabled"`
	MultipleFaceDetectionFlag     bool           `json:"multiple_face_detection_flag" db:"multiple_face_detection_flag"`
	NoFaceDetectionFlag           bool           `json:"no_face_detection_flag" db:"no_face_detection_flag"`
	IPWhitelistCIDR               []string       `json:"ip_whitelist_cidr" db:"ip_whitelist_cidr" validate:"omitempty,max=100,dive,cidr|ip"`
	SingleSessionLock             bool           `json:"single_session_lock" db:"single_session_lock"`
	RequireSafeExamBrowser        bool           `json:"require_safe_exam_browser" db:"require_safe_exam_browser"`
	SEBConfigKey                  *string        `json:"seb_config_key" db:"seb_config_key" validate:"omitempty,max=255"`
}

type ExperienceConfig struct {
	ShowCountdownTimer            bool           `json:"show_countdown_timer" db:"show_countdown_timer"`
	TimerWarningThresholdsMinutes []int32        `json:"timer_warning_thresholds_minutes" db:"timer_warning_thresholds_minutes" validate:"max=10,dive,gte=1,lte=1440"`
	ShowProgressIndicator         bool           `json:"show_progress_indicator" db:"show_progress_indicator"`
	CalculatorType                CalculatorType `json:"calculator_type" db:"calculator_type" validate:"required,oneof=none basic scientific"`
	ScratchpadEnabled             bool           `json:"scratchpad_enabled" db:"scratchpad_enabled"`
	FormulaSheetURL               *string        `json:"formula_sheet_url" db:"formula_sheet_url" validate:"omitempty,http_url,max=2048"`
	DarkModeAllowed               bool           `json:"dark_mode_allowed" db:"dark_mode_allowed"`
}

type CodingConfig struct {
	// AllowedLanguageIDs restricts the languages candidates may use; empty means all.
	AllowedLanguageIDs        []uuid.UUID `json:"allowed_language_ids" db:"allowed_language_ids" validate:"omitempty,max=50,unique"`
	CodeRunLimitPerQuestion   int         `json:"code_run_limit_per_question" db:"code_run_limit_per_question" validate:"gte=0,lte=1000"`
	EditorAutocompleteEnabled bool        `json:"editor_autocomplete_enabled" db:"editor_autocomplete_enabled"`
	AllowCustomTestInput      bool        `json:"allow_custom_test_input" db:"allow_custom_test_input"`
	ShowFailedTestcaseDiff    bool        `json:"show_failed_testcase_diff" db:"show_failed_testcase_diff"`
	DefaultTimeoutMs          int         `json:"default_timeout_ms" db:"default_timeout_ms" validate:"gte=100,lte=60000"`
	DefaultMemoryLimitMb      int         `json:"default_memory_limit_mb" db:"default_memory_limit_mb" validate:"gte=16,lte=4096"`
}

type ResultsConfig struct {
	ResultsReleaseMode         ResultsReleaseMode `json:"results_release_mode" db:"results_release_mode" validate:"required,oneof=instant scheduled manual"`
	ResultsScheduledTime       *time.Time         `json:"results_scheduled_time" db:"results_scheduled_time"`
	SolutionVisibility         SolutionVisibility `json:"solution_visibility" db:"solution_visibility" validate:"required,oneof=never after_submission after_exam_closes"`
	ShowQuestionBreakdown      bool               `json:"show_question_breakdown" db:"show_question_breakdown"`
	ShowClassPercentile        bool               `json:"show_class_percentile" db:"show_class_percentile"`
	CertificateEnabled         bool               `json:"certificate_enabled" db:"certificate_enabled"`
	CertificateMinScorePercent *float64           `json:"certificate_min_score_percent" db:"certificate_min_score_percent" validate:"omitempty,gte=0,lte=100"`
}

type AttemptPolicy struct {
	MaxAttempts             int             `json:"max_attempts" db:"max_attempts" validate:"gte=1,lte=100"`
	AttemptCooldownHours    int             `json:"attempt_cooldown_hours" db:"attempt_cooldown_hours" validate:"gte=0,lte=8760"`
	RetakeScoreRule         RetakeScoreRule `json:"retake_score_rule" db:"retake_score_rule" validate:"required,oneof=best_attempt latest_attempt average_attempt"`
	AllowResumeOnDisconnect bool            `json:"allow_resume_on_disconnect" db:"allow_resume_on_disconnect"`
}

// Section is an ordered block of questions of a single family. Override
// fields set to nil inherit the exam configuration.
type Section struct {
	ID           uuid.UUID   `json:"id" db:"id"`
	ExamID       uuid.UUID   `json:"exam_id" db:"exam_id"`
	Name         string      `json:"name" db:"name"`
	Description  *string     `json:"description,omitempty" db:"description"`
	Instructions *string     `json:"instructions,omitempty" db:"instructions"`
	SectionType  SectionType `json:"section_type" db:"section_type"`
	Position     int         `json:"position" db:"position"`

	// DurationMinutes is the section's own time limit. Only enforced when
	// the exam has per_section_timing enabled.
	DurationMinutes *int     `json:"duration_minutes,omitempty" db:"duration_minutes"`
	MarksWeightage  *float64 `json:"marks_weightage,omitempty" db:"marks_weightage"`
	CutoffMarks     *float64 `json:"cutoff_marks,omitempty" db:"cutoff_marks"`

	ShuffleQuestions  *bool `json:"shuffle_questions" db:"shuffle_questions"`
	ShuffleOptions    *bool `json:"shuffle_options" db:"shuffle_options"`
	QuestionPickCount *int  `json:"question_pick_count,omitempty" db:"question_pick_count"`

	// UnlockAfterMinutes keeps the section locked until this many minutes
	// into the attempt.
	UnlockAfterMinutes *int `json:"unlock_after_minutes,omitempty" db:"unlock_after_minutes"`
	// LockOnComplete seals the section once the candidate leaves it.
	LockOnComplete bool `json:"lock_on_complete" db:"lock_on_complete"`

	Prerequisites []Prerequisite `json:"prerequisites" db:"-"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Prerequisite gates a section on the completion of another section,
// optionally with a minimum score.
type Prerequisite struct {
	SectionID       uuid.UUID `json:"section_id"`
	MinScorePercent *float64  `json:"min_score_percent,omitempty"`
}

// SectionStats are aggregates over a section's questions.
type SectionStats struct {
	QuestionCount int
	TotalPoints   float64
	MinPoints     float64
	MaxPoints     float64
	// NegativeMarked counts questions with negative_points > 0.
	NegativeMarked int
}
