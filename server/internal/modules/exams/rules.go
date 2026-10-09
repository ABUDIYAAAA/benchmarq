package exams

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// This file is the exam rules engine: pure functions over an exam's
// configuration and sections. It has no I/O, so it is exhaustively unit
// tested and is shared by write-time validation, the readiness report and
// publishing.

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Issue is one finding of a validation pass. Path points at the offending
// field, e.g. "config.schedule.end_time" or "sections.<id>.duration_minutes".
type Issue struct {
	Severity Severity `json:"severity"`
	Path     string   `json:"path"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
}

type issues []Issue

func (is *issues) errorf(path, code, format string, args ...any) {
	*is = append(*is, Issue{Severity: SeverityError, Path: path, Code: code, Message: fmt.Sprintf(format, args...)})
}

func (is *issues) warnf(path, code, format string, args ...any) {
	*is = append(*is, Issue{Severity: SeverityWarning, Path: path, Code: code, Message: fmt.Sprintf(format, args...)})
}

// missingf reports a value that must be filled in before publishing. While
// drafting it is only a warning, so authors can save work in any order.
func (is *issues) missingf(publishing bool, path, format string, args ...any) {
	if publishing {
		is.errorf(path, "required", format, args...)
	} else {
		is.warnf(path, "required", format, args...)
	}
}

func hasErrors(list []Issue) bool {
	return slices.ContainsFunc(list, func(i Issue) bool { return i.Severity == SeverityError })
}

func splitIssues(list []Issue) (errs, warnings []Issue) {
	for _, i := range list {
		if i.Severity == SeverityError {
			errs = append(errs, i)
		} else {
			warnings = append(warnings, i)
		}
	}
	return errs, warnings
}

// issueFields flattens error issues into the path -> message map used by
// validation responses. Several messages on one path are joined.
func issueFields(list []Issue) map[string]string {
	fields := make(map[string]string)
	for _, i := range list {
		if i.Severity != SeverityError {
			continue
		}
		if prev, ok := fields[i.Path]; ok {
			fields[i.Path] = prev + "; " + i.Message
		} else {
			fields[i.Path] = i.Message
		}
	}
	return fields
}

func sectionPath(id uuid.UUID, field string) string {
	return "sections." + id.String() + "." + field
}

// ---------------------------------------------------------------------------
// Effective section rules
// ---------------------------------------------------------------------------

// EffectiveRules are the rules a section actually runs with once exam-level
// defaults, section overrides and the progression mode are combined.
type EffectiveRules struct {
	ShuffleQuestions bool `json:"shuffle_questions"`
	ShuffleOptions   bool `json:"shuffle_options"`
	// QuestionPickCount is the number of questions drawn from the section's
	// pool for each candidate; nil means every question is served.
	QuestionPickCount *int `json:"question_pick_count"`
	// DurationMinutes is the section timer; nil means the section shares
	// the exam-wide timer.
	DurationMinutes    *int `json:"duration_minutes"`
	UnlockAfterMinutes *int `json:"unlock_after_minutes"`
	LockOnComplete     bool `json:"lock_on_complete"`
	// Prerequisites includes the previous section implied by linear modes.
	// When shuffle_sections is on, that implied link follows each candidate's
	// shuffled order and is resolved at attempt time instead.
	Prerequisites []Prerequisite `json:"prerequisites"`
	// LockedAtStart reports whether the section is unavailable when an
	// attempt begins.
	LockedAtStart bool `json:"locked_at_start"`
}

// ResolveRules computes effective rules for sections, which must be sorted by
// position.
func ResolveRules(cfg *ExamConfig, sections []Section) map[uuid.UUID]EffectiveRules {
	out := make(map[uuid.UUID]EffectiveRules, len(sections))
	mode := cfg.Navigation.SectionProgressionMode

	for i, s := range sections {
		r := EffectiveRules{
			ShuffleQuestions:   boolOr(s.ShuffleQuestions, cfg.Randomization.ShuffleQuestions),
			ShuffleOptions:     boolOr(s.ShuffleOptions, cfg.Randomization.ShuffleOptions),
			QuestionPickCount:  effectivePickCount(cfg, &s),
			UnlockAfterMinutes: positiveOrNil(s.UnlockAfterMinutes),
			LockOnComplete:     s.LockOnComplete || mode.sealsCompletedSections(),
		}
		if cfg.Schedule.PerSectionTiming {
			r.DurationMinutes = s.DurationMinutes
		}

		prereqs := slices.Clone(s.Prerequisites)
		if mode.isLinear() && i > 0 && !cfg.Randomization.ShuffleSections {
			prev := sections[i-1].ID
			if !slices.ContainsFunc(prereqs, func(p Prerequisite) bool { return p.SectionID == prev }) {
				prereqs = append([]Prerequisite{{SectionID: prev}}, prereqs...)
			}
		}
		if prereqs == nil {
			prereqs = []Prerequisite{}
		}
		r.Prerequisites = prereqs
		r.LockedAtStart = len(prereqs) > 0 || r.UnlockAfterMinutes != nil ||
			(mode.isLinear() && cfg.Randomization.ShuffleSections && i > 0)

		out[s.ID] = r
	}
	return out
}

func effectivePickCount(cfg *ExamConfig, s *Section) *int {
	if s.QuestionPickCount != nil {
		return s.QuestionPickCount
	}
	if cfg.Randomization.EnableQuestionPool {
		return cfg.Randomization.PoolSampleCount
	}
	return nil
}

// SectionMaxMarks is the maximum score a candidate can earn in the section.
// Pooled sections require uniform points, so pick * points is exact.
func SectionMaxMarks(rules EffectiveRules, stats SectionStats) float64 {
	if rules.QuestionPickCount != nil && *rules.QuestionPickCount < stats.QuestionCount {
		return round2(float64(*rules.QuestionPickCount) * stats.MaxPoints)
	}
	return round2(stats.TotalPoints)
}

// ---------------------------------------------------------------------------
// Prerequisite graph
// ---------------------------------------------------------------------------

// ValidatePrerequisites checks that prerequisites reference sibling sections,
// that score gates sit on auto-gradable sections, and that the graph is acyclic.
func ValidatePrerequisites(sections []Section) []Issue {
	var is issues
	byID := make(map[uuid.UUID]*Section, len(sections))
	for i := range sections {
		byID[sections[i].ID] = &sections[i]
	}

	for _, s := range sections {
		seen := make(map[uuid.UUID]bool, len(s.Prerequisites))
		for i, p := range s.Prerequisites {
			path := sectionPath(s.ID, fmt.Sprintf("prerequisites[%d].section_id", i))
			target, ok := byID[p.SectionID]
			switch {
			case p.SectionID == s.ID:
				is.errorf(path, "prerequisite_self", "a section cannot be its own prerequisite")
			case !ok:
				is.errorf(path, "prerequisite_unknown", "prerequisite section does not belong to this exam")
			case seen[p.SectionID]:
				is.errorf(path, "prerequisite_duplicate", "prerequisite %q is listed more than once", target.Name)
			case p.MinScorePercent != nil && !target.SectionType.autoGradable():
				is.errorf(path, "prerequisite_score_needs_autograde",
					"%q is a %s section and is graded manually, so it cannot gate on a minimum score", target.Name, target.SectionType)
			}
			seen[p.SectionID] = true
		}
	}

	if cycle := FindPrerequisiteCycle(sections); cycle != nil {
		names := make([]string, len(cycle))
		for i, id := range cycle {
			names[i] = byID[id].Name
		}
		is.errorf(sectionPath(cycle[0], "prerequisites"), "prerequisite_cycle",
			"prerequisites form a cycle: %s", strings.Join(names, " → "))
	}
	return is
}

// FindPrerequisiteCycle returns one cycle in the prerequisite graph as a list
// of section IDs whose first and last elements are equal, or nil if the graph
// is a DAG. It is a three-colour depth-first search, O(V+E), visiting sections
// in position order so the reported cycle is deterministic.
func FindPrerequisiteCycle(sections []Section) []uuid.UUID {
	const (
		white = iota // unvisited
		grey         // on the current DFS path
		black        // fully explored
	)
	edges := make(map[uuid.UUID][]uuid.UUID, len(sections))
	color := make(map[uuid.UUID]int, len(sections))
	for _, s := range sections {
		color[s.ID] = white
		for _, p := range s.Prerequisites {
			edges[s.ID] = append(edges[s.ID], p.SectionID)
		}
	}

	var path []uuid.UUID
	var visit func(id uuid.UUID) []uuid.UUID
	visit = func(id uuid.UUID) []uuid.UUID {
		color[id] = grey
		path = append(path, id)
		for _, next := range edges[id] {
			c, known := color[next]
			if !known {
				continue // dangling reference, reported by ValidatePrerequisites
			}
			switch c {
			case grey:
				start := slices.Index(path, next)
				return append(slices.Clone(path[start:]), next)
			case white:
				if cycle := visit(next); cycle != nil {
					return cycle
				}
			}
		}
		path = path[:len(path)-1]
		color[id] = black
		return nil
	}

	for _, s := range sections {
		if color[s.ID] == white {
			if cycle := visit(s.ID); cycle != nil {
				return cycle
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Configuration rules
// ---------------------------------------------------------------------------

// ValidateConfig checks cross-field consistency of a configuration on its
// own. Field-level ranges are covered by struct tags. Contradictions are
// always errors; values that are merely missing are errors only when
// publishing.
func ValidateConfig(cfg *ExamConfig, publishing bool) []Issue {
	var is issues
	sc := cfg.Schedule
	duration := sc.TotalDurationMinutes

	// Schedule window
	switch sc.ScheduleType {
	case ScheduleFixedWindow, ScheduleFlexibleWindow:
		if sc.StartTime == nil {
			is.missingf(publishing, "config.schedule.start_time", "start_time is required for %s exams", sc.ScheduleType)
		}
		if sc.EndTime == nil {
			is.missingf(publishing, "config.schedule.end_time", "end_time is required for %s exams", sc.ScheduleType)
		}
	}
	if sc.StartTime != nil && sc.EndTime != nil {
		window := sc.EndTime.Sub(*sc.StartTime)
		switch {
		case window <= 0:
			is.errorf("config.schedule.end_time", "window_inverted", "end_time must be after start_time")
		case window.Minutes() < float64(duration):
			is.errorf("config.schedule.end_time", "window_too_short",
				"the %d-minute window is shorter than the %d-minute exam duration", int(window.Minutes()), duration)
		case sc.ScheduleType == ScheduleFixedWindow && float64(sc.LateJoinGraceMinutes) >= window.Minutes():
			is.errorf("config.schedule.late_join_grace_minutes", "grace_exceeds_window", "late join grace must be shorter than the exam window")
		}
	}
	if sc.ScheduleType == ScheduleFixedWindow && sc.LateJoinGraceMinutes >= duration {
		is.errorf("config.schedule.late_join_grace_minutes", "grace_exceeds_duration",
			"late join grace (%d min) must be shorter than the exam duration (%d min)", sc.LateJoinGraceMinutes, duration)
	}
	if sc.MinTimeBeforeSubmissionMinutes >= duration {
		is.errorf("config.schedule.min_time_before_submission_minutes", "exceeds_duration",
			"minimum time before submission must be shorter than the exam duration")
	}

	// Pausing
	if sc.AllowCandidatePause {
		if sc.MaxPauseCount == 0 {
			is.errorf("config.schedule.max_pause_count", "pause_count_zero", "allow at least one pause when pausing is enabled")
		}
		if sc.MaxTotalPauseMinutes == 0 {
			is.errorf("config.schedule.max_total_pause_minutes", "pause_budget_zero", "set a pause time budget when pausing is enabled")
		}
	} else if sc.MaxPauseCount > 0 || sc.MaxTotalPauseMinutes > 0 {
		is.warnf("config.schedule.allow_candidate_pause", "pause_limits_ignored", "pause limits are ignored while allow_candidate_pause is off")
	}
	if sc.PerQuestionTiming {
		is.warnf("config.schedule.per_question_timing", "not_supported", "per-question time limits are not supported yet; this flag has no effect")
	}

	// Navigation
	nav := cfg.Navigation
	if nav.SectionProgressionMode == ProgressionTimedSequential && !sc.PerSectionTiming {
		is.errorf("config.navigation.section_progression_mode", "requires_section_timing",
			"timed_sequential progression requires per_section_timing")
	}
	if nav.QuestionNavigationMode == NavigationSequentialOnly && nav.AllowRevisitingQuestions {
		is.warnf("config.navigation.allow_revisiting_questions", "contradicts_navigation",
			"sequential_only navigation does not allow going back; allow_revisiting_questions is ignored")
	}

	// Randomization
	rnd := cfg.Randomization
	if rnd.EnableQuestionPool && rnd.PoolSampleCount == nil {
		is.warnf("config.randomization.pool_sample_count", "pool_without_default",
			"question pooling is on but pool_sample_count is not set; only sections with question_pick_count will be pooled")
	}
	if !rnd.EnableQuestionPool && rnd.PoolSampleCount != nil {
		is.warnf("config.randomization.pool_sample_count", "pool_disabled", "pool_sample_count is ignored while enable_question_pool is off")
	}

	// Scoring
	if cfg.Scoring.BlindGradingEnabled && cfg.Scoring.GradingStrategy == GradingAutomated {
		is.warnf("config.scoring.blind_grading_enabled", "no_manual_grading", "blind grading has no effect with fully automated grading")
	}

	// Proctoring
	pr := cfg.Proctoring
	if pr.RequireSafeExamBrowser && (pr.SEBConfigKey == nil || strings.TrimSpace(*pr.SEBConfigKey) == "") {
		is.missingf(publishing, "config.proctoring.seb_config_key", "seb_config_key is required when Safe Exam Browser is enforced")
	}
	mediaProctored := pr.ProctoringMode == ProctoringAI || pr.ProctoringMode == ProctoringLive
	for _, f := range []struct {
		on   bool
		name string
	}{
		{pr.WebcamSnapshotsEnabled, "webcam_snapshots_enabled"},
		{pr.AudioMonitoringEnabled, "audio_monitoring_enabled"},
		{pr.MultipleFaceDetectionFlag, "multiple_face_detection_flag"},
		{pr.NoFaceDetectionFlag, "no_face_detection_flag"},
	} {
		if f.on && !mediaProctored {
			is.errorf("config.proctoring."+f.name, "requires_media_proctoring",
				"%s requires proctoring_mode ai_proctored or live_proctored", f.name)
		}
	}
	if (pr.MultipleFaceDetectionFlag || pr.NoFaceDetectionFlag) && pr.ProctoringMode == ProctoringAI && !pr.WebcamSnapshotsEnabled {
		is.errorf("config.proctoring.webcam_snapshots_enabled", "face_detection_needs_webcam", "face detection requires webcam snapshots")
	}
	if !pr.TrackTabFocus && pr.MaxTabSwitchCount > 0 {
		is.warnf("config.proctoring.max_tab_switch_count", "tab_tracking_off", "tab switch limits are ignored while track_tab_focus is off")
	}

	// Candidate experience
	thresholds := cfg.Experience.TimerWarningThresholdsMinutes
	seen := make(map[int32]bool, len(thresholds))
	for i, t := range thresholds {
		path := fmt.Sprintf("config.experience.timer_warning_thresholds_minutes[%d]", i)
		if seen[t] {
			is.errorf(path, "duplicate", "timer warning at %d minutes is listed twice", t)
		}
		seen[t] = true
		if int(t) >= duration {
			is.warnf(path, "exceeds_duration", "a warning at %d minutes will never fire in a %d-minute exam", t, duration)
		}
	}

	// Results
	res := cfg.Results
	if res.ResultsReleaseMode == ResultsScheduled {
		if res.ResultsScheduledTime == nil {
			is.missingf(publishing, "config.results.results_scheduled_time", "results_scheduled_time is required for scheduled release")
		} else if sc.EndTime != nil && res.ResultsScheduledTime.Before(*sc.EndTime) {
			is.warnf("config.results.results_scheduled_time", "before_exam_end", "results will be released before the exam window closes")
		}
	}
	if res.SolutionVisibility == SolutionAfterExamCloses && sc.EndTime == nil {
		is.missingf(publishing, "config.schedule.end_time", "solution_visibility after_exam_closes requires the exam to have an end_time")
	}
	if res.CertificateEnabled && res.CertificateMinScorePercent == nil {
		is.missingf(publishing, "config.results.certificate_min_score_percent", "set a minimum score for certificates")
	}

	// Attempts
	if cfg.Attempts.MaxAttempts == 1 && cfg.Attempts.AttemptCooldownHours > 0 {
		is.warnf("config.attempts.attempt_cooldown_hours", "single_attempt", "cooldown has no effect when only one attempt is allowed")
	}

	return is
}

// ValidateSectionAgainstConfig checks the parts of a section that only depend
// on the exam configuration. It runs whenever a section is written.
func ValidateSectionAgainstConfig(cfg *ExamConfig, s *Section) []Issue {
	var is issues
	duration := cfg.Schedule.TotalDurationMinutes
	if s.DurationMinutes != nil && *s.DurationMinutes > duration {
		is.errorf(sectionPath(s.ID, "duration_minutes"), "exceeds_exam_duration",
			"section duration (%d min) exceeds the exam duration (%d min)", *s.DurationMinutes, duration)
	}
	if s.UnlockAfterMinutes != nil && *s.UnlockAfterMinutes >= duration {
		is.errorf(sectionPath(s.ID, "unlock_after_minutes"), "never_unlocks",
			"the section would unlock after the %d-minute exam has ended", duration)
	}
	return is
}

// ---------------------------------------------------------------------------
// Publish readiness
// ---------------------------------------------------------------------------

// Blueprint is everything needed to judge whether an exam can be published.
type Blueprint struct {
	Exam     *Exam
	Config   *ExamConfig
	Sections []Section // sorted by position
	Stats    map[uuid.UUID]SectionStats
}

type Readiness struct {
	Ready                bool    `json:"ready"`
	TotalMarks           float64 `json:"total_marks"`
	TotalQuestions       int     `json:"total_questions"`
	ServedQuestions      int     `json:"served_questions"`
	TotalDurationMinutes int     `json:"total_duration_minutes"`
	Errors               []Issue `json:"errors"`
	Warnings             []Issue `json:"warnings"`
}

// AssessReadiness runs every configuration, structure, timing, pooling and
// scoring rule and reports whether the exam can be published.
func AssessReadiness(bp Blueprint) Readiness {
	cfg := bp.Config
	is := issues(ValidateConfig(cfg, true))
	is = append(is, ValidatePrerequisites(bp.Sections)...)
	rules := ResolveRules(cfg, bp.Sections)
	mode := cfg.Navigation.SectionProgressionMode

	if len(bp.Sections) == 0 {
		is.errorf("sections", "no_sections", "add at least one section")
	}

	var (
		totalMarks      float64
		totalQuestions  int
		servedQuestions int
		sectionMinutes  int
		untimedSections int
		hasSubjective   bool
		hasCoding       bool
		negativeMarked  int
		openAtStart     bool
	)
	position := make(map[uuid.UUID]int, len(bp.Sections))
	for i, s := range bp.Sections {
		position[s.ID] = i
	}

	for i, s := range bp.Sections {
		stats := bp.Stats[s.ID]
		r := rules[s.ID]
		totalQuestions += stats.QuestionCount
		negativeMarked += stats.NegativeMarked
		hasSubjective = hasSubjective || s.SectionType == SectionSubjective
		hasCoding = hasCoding || s.SectionType == SectionCoding
		if !r.LockedAtStart {
			openAtStart = true
		}

		is = append(is, ValidateSectionAgainstConfig(cfg, &s)...)

		if stats.QuestionCount == 0 {
			is.errorf(sectionPath(s.ID, "questions"), "empty_section", "section %q has no questions", s.Name)
		}

		// Pooling
		served := stats.QuestionCount
		if pick := r.QuestionPickCount; pick != nil && stats.QuestionCount > 0 {
			switch {
			case *pick > stats.QuestionCount:
				is.errorf(sectionPath(s.ID, "question_pick_count"), "pool_too_small",
					"section %q draws %d questions but only has %d", s.Name, *pick, stats.QuestionCount)
			case stats.MinPoints != stats.MaxPoints:
				is.errorf(sectionPath(s.ID, "question_pick_count"), "pool_points_not_uniform",
					"questions in pooled section %q must all carry the same points so every candidate faces the same maximum score", s.Name)
			case *pick == stats.QuestionCount:
				is.warnf(sectionPath(s.ID, "question_pick_count"), "pool_has_no_effect",
					"section %q draws all of its %d questions; pooling only changes their order", s.Name, *pick)
			}
			served = min(*pick, stats.QuestionCount)
		}
		servedQuestions += served

		maxMarks := SectionMaxMarks(r, stats)
		totalMarks += maxMarks
		if s.CutoffMarks != nil && *s.CutoffMarks > maxMarks {
			is.errorf(sectionPath(s.ID, "cutoff_marks"), "cutoff_exceeds_max",
				"cutoff of %.2f exceeds the section's maximum of %.2f marks", *s.CutoffMarks, maxMarks)
		}

		// Timing
		if cfg.Schedule.PerSectionTiming {
			if s.DurationMinutes == nil {
				untimedSections++
				is.errorf(sectionPath(s.ID, "duration_minutes"), "required",
					"section %q needs a duration because per_section_timing is on", s.Name)
			} else {
				sectionMinutes += *s.DurationMinutes
			}
		} else if s.DurationMinutes != nil {
			is.warnf(sectionPath(s.ID, "duration_minutes"), "section_timing_off",
				"duration of section %q is ignored while per_section_timing is off", s.Name)
		}

		// Ordering constraints
		for j, p := range s.Prerequisites {
			path := sectionPath(s.ID, fmt.Sprintf("prerequisites[%d].section_id", j))
			if cfg.Randomization.ShuffleSections {
				is.errorf(path, "prerequisite_with_shuffle", "explicit prerequisites cannot be combined with shuffle_sections")
				continue
			}
			if pos, ok := position[p.SectionID]; ok && mode.isLinear() && pos > i {
				is.errorf(path, "prerequisite_deadlock",
					"in %s mode a section cannot depend on a later section; candidates would never reach it", mode)
			}
		}
	}

	if cfg.Schedule.PerSectionTiming && untimedSections == 0 && len(bp.Sections) > 0 {
		total := cfg.Schedule.TotalDurationMinutes
		switch {
		case sectionMinutes > total:
			is.errorf("config.schedule.total_duration_minutes", "sections_exceed_duration",
				"section durations add up to %d minutes, more than the %d-minute exam duration", sectionMinutes, total)
		case sectionMinutes < total && mode == ProgressionTimedSequential:
			is.warnf("config.schedule.total_duration_minutes", "unallocated_time",
				"%d minutes of exam time are not allocated to any section", total-sectionMinutes)
		}
	}

	if len(bp.Sections) > 0 && !openAtStart {
		is.warnf("sections", "nothing_open_at_start", "every section is locked when the attempt starts; candidates will have to wait")
	}

	if hasSubjective && cfg.Scoring.GradingStrategy == GradingAutomated {
		is.errorf("config.scoring.grading_strategy", "subjective_needs_manual",
			"subjective sections need manual or hybrid grading")
	}
	if negativeMarked > 0 && !cfg.Scoring.NegativeMarkingEnabled {
		is.warnf("config.scoring.negative_marking_enabled", "negative_points_ignored",
			"%d question(s) define negative points, which are ignored while negative marking is off", negativeMarked)
	}
	if hasCoding && len(cfg.Coding.AllowedLanguageIDs) == 0 {
		is.warnf("config.coding.allowed_language_ids", "all_languages", "no language restriction is set; candidates may use any supported language")
	}

	totalMarks = round2(totalMarks)
	if bp.Exam.PassingMarks != nil && *bp.Exam.PassingMarks > totalMarks {
		is.errorf("exam.passing_marks", "exceeds_total",
			"passing marks (%.2f) exceed the exam's total of %.2f", *bp.Exam.PassingMarks, totalMarks)
	}

	errs, warnings := splitIssues(is)
	if errs == nil {
		errs = []Issue{}
	}
	if warnings == nil {
		warnings = []Issue{}
	}
	return Readiness{
		Ready:                len(errs) == 0,
		TotalMarks:           totalMarks,
		TotalQuestions:       totalQuestions,
		ServedQuestions:      servedQuestions,
		TotalDurationMinutes: cfg.Schedule.TotalDurationMinutes,
		Errors:               errs,
		Warnings:             warnings,
	}
}

func boolOr(override *bool, fallback bool) bool {
	if override != nil {
		return *override
	}
	return fallback
}

func positiveOrNil(v *int) *int {
	if v == nil || *v <= 0 {
		return nil
	}
	return v
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
