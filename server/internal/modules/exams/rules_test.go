package exams

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// defaultConfig mirrors the column defaults in exam_configurations.
func defaultConfig() *ExamConfig {
	return &ExamConfig{
		Schedule: ScheduleConfig{
			ScheduleType:                   ScheduleOpenEnded,
			TotalDurationMinutes:           60,
			LateJoinGraceMinutes:           15,
			MinTimeBeforeSubmissionMinutes: 5,
			AutoSubmitOnExpiry:             true,
		},
		Navigation: NavigationConfig{
			SectionProgressionMode: ProgressionFreeNavigation,
			QuestionNavigationMode: NavigationFreeJump,
		},
		Scoring: ScoringConfig{
			MultiSelectScoringMode: "partial_with_penalty",
			PassingPercentage:      50,
			GradingStrategy:        GradingAutomated,
		},
		Proctoring: ProctoringConfig{ProctoringMode: ProctoringDisabled, WebcamSnapshotIntervalSeconds: 60, TrackTabFocus: true},
		Experience: ExperienceConfig{CalculatorType: "none", TimerWarningThresholdsMinutes: []int32{15, 5, 1}},
		Coding:     CodingConfig{DefaultTimeoutMs: 2000, DefaultMemoryLimitMb: 256},
		Results:    ResultsConfig{ResultsReleaseMode: "instant", SolutionVisibility: "after_submission"},
		Attempts:   AttemptPolicy{MaxAttempts: 1, RetakeScoreRule: "best_attempt"},
	}
}

func section(name string, typ SectionType, pos int) Section {
	return Section{ID: uuid.New(), Name: name, SectionType: typ, Position: pos, Prerequisites: []Prerequisite{}}
}

func ptr[T any](v T) *T { return &v }

func codes(list []Issue) []string {
	out := make([]string, len(list))
	for i, is := range list {
		out[i] = is.Code
	}
	return out
}

func requireCode(t *testing.T, list []Issue, code string) {
	t.Helper()
	if !slices.Contains(codes(list), code) {
		t.Fatalf("expected issue %q, got %v", code, codes(list))
	}
}

func refuseCode(t *testing.T, list []Issue, code string) {
	t.Helper()
	if slices.Contains(codes(list), code) {
		t.Fatalf("did not expect issue %q, got %v", code, codes(list))
	}
}

// ---------------------------------------------------------------------------

func TestFindPrerequisiteCycle(t *testing.T) {
	a, b, c, d := section("A", SectionObjective, 0), section("B", SectionObjective, 1), section("C", SectionObjective, 2), section("D", SectionObjective, 3)

	t.Run("acyclic diamond", func(t *testing.T) {
		b.Prerequisites = []Prerequisite{{SectionID: a.ID}}
		c.Prerequisites = []Prerequisite{{SectionID: a.ID}}
		d.Prerequisites = []Prerequisite{{SectionID: b.ID}, {SectionID: c.ID}}
		if cycle := FindPrerequisiteCycle([]Section{a, b, c, d}); cycle != nil {
			t.Fatalf("unexpected cycle %v", cycle)
		}
	})

	t.Run("three-node cycle", func(t *testing.T) {
		a.Prerequisites = []Prerequisite{{SectionID: c.ID}}
		b.Prerequisites = []Prerequisite{{SectionID: a.ID}}
		c.Prerequisites = []Prerequisite{{SectionID: b.ID}}
		cycle := FindPrerequisiteCycle([]Section{a, b, c})
		if len(cycle) != 4 || cycle[0] != cycle[len(cycle)-1] {
			t.Fatalf("expected closed 3-cycle, got %v", cycle)
		}
	})

	t.Run("dangling reference is not a cycle", func(t *testing.T) {
		a.Prerequisites = []Prerequisite{{SectionID: uuid.New()}}
		if cycle := FindPrerequisiteCycle([]Section{a}); cycle != nil {
			t.Fatalf("unexpected cycle %v", cycle)
		}
	})
}

func TestValidatePrerequisites(t *testing.T) {
	obj, subj := section("Aptitude", SectionObjective, 0), section("Essay", SectionSubjective, 1)
	target := section("Final", SectionObjective, 2)

	target.Prerequisites = []Prerequisite{
		{SectionID: obj.ID, MinScorePercent: ptr(60.0)},
		{SectionID: subj.ID, MinScorePercent: ptr(50.0)},
		{SectionID: obj.ID},
		{SectionID: uuid.New()},
		{SectionID: target.ID},
	}
	is := ValidatePrerequisites([]Section{obj, subj, target})
	for _, code := range []string{"prerequisite_score_needs_autograde", "prerequisite_duplicate", "prerequisite_unknown", "prerequisite_self"} {
		requireCode(t, is, code)
	}

	a, b := section("A", SectionObjective, 0), section("B", SectionObjective, 1)
	a.Prerequisites = []Prerequisite{{SectionID: b.ID}}
	b.Prerequisites = []Prerequisite{{SectionID: a.ID}}
	is = ValidatePrerequisites([]Section{a, b})
	requireCode(t, is, "prerequisite_cycle")
	if !strings.Contains(is[0].Message, "A → B → A") {
		t.Fatalf("cycle message should name the path, got %q", is[0].Message)
	}
}

func TestResolveRules(t *testing.T) {
	cfg := defaultConfig()
	cfg.Randomization.ShuffleQuestions = true
	cfg.Randomization.ShuffleOptions = true
	cfg.Randomization.EnableQuestionPool = true
	cfg.Randomization.PoolSampleCount = ptr(10)

	a, b, c := section("A", SectionObjective, 0), section("B", SectionObjective, 1), section("C", SectionObjective, 2)
	b.ShuffleQuestions = ptr(false) // override
	b.QuestionPickCount = ptr(3)    // override
	b.DurationMinutes = ptr(20)
	c.UnlockAfterMinutes = ptr(30)
	sections := []Section{a, b, c}

	t.Run("free navigation inherits and overrides", func(t *testing.T) {
		r := ResolveRules(cfg, sections)
		if !r[a.ID].ShuffleQuestions || r[b.ID].ShuffleQuestions {
			t.Fatal("section shuffle override not applied")
		}
		if *r[a.ID].QuestionPickCount != 10 || *r[b.ID].QuestionPickCount != 3 {
			t.Fatal("pool pick count not resolved")
		}
		if r[b.ID].DurationMinutes != nil {
			t.Fatal("section timer must be ignored without per_section_timing")
		}
		if r[a.ID].LockedAtStart || r[b.ID].LockedAtStart || !r[c.ID].LockedAtStart {
			t.Fatal("only the time-gated section should start locked")
		}
		if r[a.ID].LockOnComplete {
			t.Fatal("free navigation should not seal sections")
		}
	})

	t.Run("linear strict chains and seals sections", func(t *testing.T) {
		strict := *cfg
		strict.Navigation.SectionProgressionMode = ProgressionLinearStrict
		strict.Schedule.PerSectionTiming = true
		r := ResolveRules(&strict, sections)
		if len(r[a.ID].Prerequisites) != 0 || r[b.ID].Prerequisites[0].SectionID != a.ID || r[c.ID].Prerequisites[0].SectionID != b.ID {
			t.Fatal("linear mode must chain each section to the previous one")
		}
		if !r[a.ID].LockOnComplete || !r[c.ID].LockOnComplete {
			t.Fatal("linear_strict must seal completed sections")
		}
		if r[b.ID].DurationMinutes == nil || *r[b.ID].DurationMinutes != 20 {
			t.Fatal("section timer should apply with per_section_timing")
		}
	})

	t.Run("linear with shuffled sections defers chaining", func(t *testing.T) {
		relaxed := *cfg
		relaxed.Navigation.SectionProgressionMode = ProgressionLinearRelaxed
		relaxed.Randomization.ShuffleSections = true
		r := ResolveRules(&relaxed, sections)
		if len(r[b.ID].Prerequisites) != 0 || !r[b.ID].LockedAtStart {
			t.Fatal("shuffled linear sections resolve order at attempt time but still start locked")
		}
	})
}

func TestValidateConfig(t *testing.T) {
	t.Run("defaults are clean", func(t *testing.T) {
		if is := ValidateConfig(defaultConfig(), true); hasErrors(is) {
			t.Fatalf("default config should be valid, got %v", is)
		}
	})

	t.Run("windowed schedules need a long enough window", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Schedule.ScheduleType = ScheduleFixedWindow
		requireCode(t, ValidateConfig(cfg, true), "required")
		if is := ValidateConfig(cfg, false); hasErrors(is) {
			t.Fatalf("missing times must not block saving a draft: %v", is)
		}

		start := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
		cfg.Schedule.StartTime = &start
		cfg.Schedule.EndTime = ptr(start.Add(30 * time.Minute))
		requireCode(t, ValidateConfig(cfg, true), "window_too_short")

		cfg.Schedule.EndTime = ptr(start.Add(-time.Minute))
		requireCode(t, ValidateConfig(cfg, true), "window_inverted")

		cfg.Schedule.EndTime = ptr(start.Add(2 * time.Hour))
		if is := ValidateConfig(cfg, true); hasErrors(is) {
			t.Fatalf("valid window rejected: %v", is)
		}
	})

	t.Run("timed sequential requires section timing", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Navigation.SectionProgressionMode = ProgressionTimedSequential
		requireCode(t, ValidateConfig(cfg, true), "requires_section_timing")
	})

	t.Run("media proctoring needs a media mode", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Proctoring.WebcamSnapshotsEnabled = true
		requireCode(t, ValidateConfig(cfg, true), "requires_media_proctoring")
		cfg.Proctoring.ProctoringMode = ProctoringAI
		refuseCode(t, ValidateConfig(cfg, true), "requires_media_proctoring")
	})

	t.Run("safe exam browser needs a key", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Proctoring.RequireSafeExamBrowser = true
		cfg.Proctoring.SEBConfigKey = ptr("  ")
		requireCode(t, ValidateConfig(cfg, true), "required")
	})

	t.Run("pausing needs limits", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Schedule.AllowCandidatePause = true
		requireCode(t, ValidateConfig(cfg, true), "pause_count_zero")
	})

	t.Run("scheduled results need a time", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Results.ResultsReleaseMode = ResultsScheduled
		requireCode(t, ValidateConfig(cfg, true), "required")
	})
}

func TestAssessReadiness(t *testing.T) {
	build := func(mutate func(bp *Blueprint)) Readiness {
		a, b := section("Quant", SectionObjective, 0), section("Verbal", SectionObjective, 1)
		bp := Blueprint{
			Exam:     &Exam{Title: "Mock"},
			Config:   defaultConfig(),
			Sections: []Section{a, b},
			Stats: map[uuid.UUID]SectionStats{
				a.ID: {QuestionCount: 10, TotalPoints: 20, MinPoints: 2, MaxPoints: 2},
				b.ID: {QuestionCount: 5, TotalPoints: 9, MinPoints: 1, MaxPoints: 3},
			},
		}
		if mutate != nil {
			mutate(&bp)
		}
		return AssessReadiness(bp)
	}

	t.Run("valid exam is ready with correct totals", func(t *testing.T) {
		r := build(nil)
		if !r.Ready || r.TotalMarks != 29 || r.TotalQuestions != 15 || r.ServedQuestions != 15 {
			t.Fatalf("unexpected readiness %+v", r)
		}
	})

	t.Run("pooling scales marks and requires uniform points", func(t *testing.T) {
		r := build(func(bp *Blueprint) { bp.Sections[0].QuestionPickCount = ptr(4) })
		if !r.Ready || r.TotalMarks != 4*2+9 || r.ServedQuestions != 4+5 {
			t.Fatalf("unexpected pooled readiness %+v", r)
		}
		r = build(func(bp *Blueprint) { bp.Sections[1].QuestionPickCount = ptr(2) })
		requireCode(t, r.Errors, "pool_points_not_uniform")
		r = build(func(bp *Blueprint) { bp.Sections[0].QuestionPickCount = ptr(11) })
		requireCode(t, r.Errors, "pool_too_small")
	})

	t.Run("per-section timing must fit the exam", func(t *testing.T) {
		r := build(func(bp *Blueprint) {
			bp.Config.Schedule.PerSectionTiming = true
			bp.Sections[0].DurationMinutes = ptr(40)
		})
		requireCode(t, r.Errors, "required")

		r = build(func(bp *Blueprint) {
			bp.Config.Schedule.PerSectionTiming = true
			bp.Sections[0].DurationMinutes = ptr(40)
			bp.Sections[1].DurationMinutes = ptr(30)
		})
		requireCode(t, r.Errors, "sections_exceed_duration")
	})

	t.Run("linear mode rejects forward prerequisites", func(t *testing.T) {
		r := build(func(bp *Blueprint) {
			bp.Config.Navigation.SectionProgressionMode = ProgressionLinearRelaxed
			bp.Sections[0].Prerequisites = []Prerequisite{{SectionID: bp.Sections[1].ID}}
		})
		requireCode(t, r.Errors, "prerequisite_deadlock")
	})

	t.Run("prerequisites cannot be combined with shuffled sections", func(t *testing.T) {
		r := build(func(bp *Blueprint) {
			bp.Config.Randomization.ShuffleSections = true
			bp.Sections[1].Prerequisites = []Prerequisite{{SectionID: bp.Sections[0].ID}}
		})
		requireCode(t, r.Errors, "prerequisite_with_shuffle")
	})

	t.Run("structural problems", func(t *testing.T) {
		r := build(func(bp *Blueprint) { bp.Stats[bp.Sections[1].ID] = SectionStats{} })
		requireCode(t, r.Errors, "empty_section")

		r = build(func(bp *Blueprint) { bp.Sections[0].CutoffMarks = ptr(25.0) })
		requireCode(t, r.Errors, "cutoff_exceeds_max")

		r = build(func(bp *Blueprint) { bp.Exam.PassingMarks = ptr(100.0) })
		requireCode(t, r.Errors, "exceeds_total")

		r = build(func(bp *Blueprint) { bp.Sections[1].SectionType = SectionSubjective })
		requireCode(t, r.Errors, "subjective_needs_manual")

		r = build(func(bp *Blueprint) { bp.Sections = nil })
		requireCode(t, r.Errors, "no_sections")
	})

	t.Run("warnings do not block publishing", func(t *testing.T) {
		r := build(func(bp *Blueprint) {
			bp.Sections[0].UnlockAfterMinutes = ptr(5)
			bp.Sections[1].UnlockAfterMinutes = ptr(5)
			bp.Stats[bp.Sections[0].ID] = SectionStats{QuestionCount: 10, TotalPoints: 20, MinPoints: 2, MaxPoints: 2, NegativeMarked: 3}
		})
		if !r.Ready {
			t.Fatalf("expected ready, got errors %v", r.Errors)
		}
		requireCode(t, r.Warnings, "nothing_open_at_start")
		requireCode(t, r.Warnings, "negative_points_ignored")
	})
}
