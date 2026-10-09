package exams

// String enums mirror the Postgres enum types defined in the migrations.
// Validation uses `oneof` tags on DTOs; keep both lists in sync with the SQL.

type ExamStatus string

const (
	StatusDraft     ExamStatus = "draft"
	StatusPublished ExamStatus = "published"
	StatusArchived  ExamStatus = "archived"
	StatusConcluded ExamStatus = "concluded"
)

type ScheduleType string

const (
	ScheduleFixedWindow    ScheduleType = "fixed_window"    // everyone starts at start_time
	ScheduleFlexibleWindow ScheduleType = "flexible_window" // start any time inside [start_time, end_time]
	ScheduleOpenEnded      ScheduleType = "open_ended"      // no window
)

// SectionProgressionMode controls how candidates move between sections.
type SectionProgressionMode string

const (
	// Sections in authored order; completed sections are sealed.
	ProgressionLinearStrict SectionProgressionMode = "linear_strict"
	// Sections in authored order; completed sections may be revisited unless
	// they set lock_on_complete.
	ProgressionLinearRelaxed SectionProgressionMode = "linear_relaxed"
	// Any unlocked section, in any order. Prerequisites and unlock offsets
	// decide what is unlocked.
	ProgressionFreeNavigation SectionProgressionMode = "free_navigation"
	// linear_strict where every section runs on its own timer and the
	// candidate is advanced automatically when it expires.
	ProgressionTimedSequential SectionProgressionMode = "timed_sequential"
)

// isLinear reports whether sections must be taken in order.
func (m SectionProgressionMode) isLinear() bool {
	return m == ProgressionLinearStrict || m == ProgressionLinearRelaxed || m == ProgressionTimedSequential
}

// sealsCompletedSections reports whether leaving a section always locks it.
func (m SectionProgressionMode) sealsCompletedSections() bool {
	return m == ProgressionLinearStrict || m == ProgressionTimedSequential
}

type QuestionNavigationMode string

const (
	NavigationFreeJump        QuestionNavigationMode = "free_jump"
	NavigationSequentialOnly  QuestionNavigationMode = "sequential_only"
	NavigationMandatoryAnswer QuestionNavigationMode = "mandatory_answer"
)

type MultiSelectScoringMode string

type ProctoringMode string

const (
	ProctoringDisabled    ProctoringMode = "disabled"
	ProctoringBrowserOnly ProctoringMode = "browser_only"
	ProctoringAI          ProctoringMode = "ai_proctored"
	ProctoringLive        ProctoringMode = "live_proctored"
)

type CalculatorType string

type ResultsReleaseMode string

const ResultsScheduled ResultsReleaseMode = "scheduled"

type SolutionVisibility string

const SolutionAfterExamCloses SolutionVisibility = "after_exam_closes"

type RetakeScoreRule string

const (
	GradingAutomated = "automated"
	GradingManual    = "manual"
	GradingHybrid    = "hybrid"
)

type SectionType string

const (
	SectionObjective  SectionType = "objective"
	SectionCoding     SectionType = "coding"
	SectionSubjective SectionType = "subjective"
)

// autoGradable reports whether scores for the section are known as soon as
// the candidate submits, which score-gated prerequisites depend on.
func (t SectionType) autoGradable() bool {
	return t == SectionObjective || t == SectionCoding
}
