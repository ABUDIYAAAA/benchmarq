package attempts

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/questions"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/google/uuid"
)

var t0 = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

func at(minutes float64) time.Time { return t0.Add(time.Duration(minutes * float64(time.Minute))) }

func testConfig() *exams.ExamConfig {
	return &exams.ExamConfig{
		Schedule: exams.ScheduleConfig{
			ScheduleType:             exams.ScheduleOpenEnded,
			TotalDurationMinutes:     60,
			AutoSubmitOnExpiry:       true,
			ReconnectionGraceSeconds: 120,
		},
		Navigation: exams.NavigationConfig{
			SectionProgressionMode:   exams.ProgressionFreeNavigation,
			QuestionNavigationMode:   exams.NavigationFreeJump,
			AllowRevisitingQuestions: true,
		},
		Attempts: exams.AttemptPolicy{MaxAttempts: 1, AllowResumeOnDisconnect: true},
	}
}

// fixture builds an engine over n sections with three questions each.
type fixture struct {
	cfg      *exams.ExamConfig
	sections []exams.Section
	limits   map[int]int // section index -> time limit in minutes
	scores   map[uuid.UUID]float64
}

func newFixture(n int) *fixture {
	f := &fixture{cfg: testConfig(), limits: map[int]int{}, scores: map[uuid.UUID]float64{}}
	for i := range n {
		f.sections = append(f.sections, exams.Section{
			ID: uuid.New(), Name: string(rune('A' + i)), SectionType: exams.SectionObjective, Position: i,
			Prerequisites: []exams.Prerequisite{},
		})
	}
	return f
}

func (f *fixture) build() *engine {
	rules := exams.ResolveRules(f.cfg, f.sections)
	infos := make(map[uuid.UUID]sectionInfo)
	var src []paperSection
	for i, s := range f.sections {
		infos[s.ID] = sectionInfo{Section: s, Rules: rules[s.ID]}
		ps := paperSection{ID: s.ID, Rules: rules[s.ID]}
		if m, ok := f.limits[i]; ok {
			ps.TimeLimitSeconds = ptr(m * 60)
		}
		for range 3 {
			ps.Questions = append(ps.Questions, paperSource{ID: uuid.New(), Points: 1})
		}
		src = append(src, ps)
	}
	states, paper := buildPaper(1, src, f.cfg.Randomization.ShuffleSections)
	a := &Attempt{
		ID: uuid.New(), Status: StatusInProgress, StartedAt: t0, TimerOriginAt: t0,
		LimitSeconds: f.cfg.Schedule.TotalDurationMinutes * 60, LastHeartbeatAt: &t0,
	}
	score := func(id uuid.UUID) (*float64, float64, error) {
		v := f.scores[id]
		return &v, 3, nil
	}
	return newEngine(f.cfg, a, infos, states, paper, nil, score)
}

func (f *fixture) id(i int) uuid.UUID { return f.sections[i].ID }

func mustConflict(t *testing.T, err error) {
	t.Helper()
	if apperr.KindOf(err) != apperr.KindConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestClockFixedWindowAndPauses(t *testing.T) {
	end := at(90)
	a := &Attempt{StartedAt: at(10), TimerOriginAt: t0, LimitSeconds: 3600, WindowEndAt: &end}

	// A late joiner on a fixed window still ends with everyone else.
	c := computeClock(a, 0, at(20))
	if !c.deadline.Equal(at(60)) || c.remaining() != 40*time.Minute {
		t.Fatalf("deadline %v remaining %v", c.deadline, c.remaining())
	}

	// A running pause pushes the deadline but never past the window.
	a.PausedAt = ptr(at(30))
	c = computeClock(a, 10*time.Minute, at(35))
	if !c.paused || !c.deadline.Equal(at(65)) || c.pauseLeft != 5*time.Minute {
		t.Fatalf("paused clock wrong: %+v", c)
	}
	c = computeClock(a, 10*time.Minute, at(50)) // budget ran out at 40
	if c.paused || !c.deadline.Equal(at(70)) {
		t.Fatalf("exhausted pause should stop the clock extending: %+v", c)
	}
	a.PausedAt, a.PausedSeconds = nil, 3600
	if c := computeClock(a, 2*time.Hour, at(20)); !c.deadline.Equal(end) {
		t.Fatalf("window end should cap the deadline, got %v", c.deadline)
	}
	if c := computeClock(a, 2*time.Hour, at(20)); c.activeElapsed(a) != 0 {
		t.Fatalf("elapsed should not go negative, got %v", c.activeElapsed(a))
	}
}

func TestLinearStrictFlow(t *testing.T) {
	f := newFixture(3)
	f.cfg.Navigation.SectionProgressionMode = exams.ProgressionLinearStrict
	e := f.build()

	mustConflict(t, e.enterSection(f.id(1), at(1))) // needs A first
	must(t, e.enterSection(f.id(0), at(1)))
	mustConflict(t, e.enterSection(f.id(1), at(2))) // A not finished
	must(t, e.finishSection(f.id(0), at(5)))
	if cur := e.current(); cur == nil || cur.SectionID != f.id(1) {
		t.Fatal("finishing a section should open the next one in linear modes")
	}
	mustConflict(t, e.enterSection(f.id(0), at(6))) // completed sections are sealed
	if st := e.state(f.id(0)); st.TimeSpentSeconds != 240 || st.Status != SectionCompleted {
		t.Fatalf("section A state wrong: %+v", st)
	}
}

func TestTimedSequentialCatchesUpAfterSilence(t *testing.T) {
	f := newFixture(3)
	f.cfg.Navigation.SectionProgressionMode = exams.ProgressionTimedSequential
	f.cfg.Schedule.PerSectionTiming = true
	f.limits = map[int]int{0: 10, 1: 10, 2: 10}
	e := f.build()
	must(t, e.enterSection(f.id(0), at(0)))

	// Nobody looks for 25 minutes: A closed at 10, B ran 10..20, C opened at 20.
	must(t, e.reconcile(at(25)))
	cur := e.current()
	if cur == nil || cur.SectionID != f.id(2) || !cur.EnteredAt.Equal(at(20)) {
		t.Fatalf("expected C open since minute 20, got %+v", cur)
	}
	for i := range 2 {
		if st := e.state(f.id(i)); st.Status != SectionCompleted || st.TimeSpentSeconds != 600 || !st.CompletedAt.Equal(at(float64(10*(i+1)))) {
			t.Fatalf("section %d not closed on time: %+v", i, st)
		}
	}
	must(t, e.reconcile(at(31)))
	if e.attempt.Status != StatusInProgress || e.current() != nil {
		t.Fatalf("after the last section closes the attempt waits for submit or expiry: %+v", e.attempt)
	}
}

func TestExpiryAutoSubmitsOrAbandons(t *testing.T) {
	for _, auto := range []bool{true, false} {
		f := newFixture(2)
		f.cfg.Schedule.AutoSubmitOnExpiry = auto
		e := f.build()
		must(t, e.enterSection(f.id(0), at(1)))
		must(t, e.reconcile(at(61)))
		want := StatusAbandoned
		if auto {
			want = StatusAutoSubmitted
		}
		a := e.attempt
		if a.Status != want || !a.SubmittedAt.Equal(at(60)) || *a.EndedReason != EndTimeExpired {
			t.Fatalf("auto=%v: attempt %+v", auto, a)
		}
		if e.state(f.id(0)).Status != SectionCompleted || e.state(f.id(1)).Status != SectionSkipped {
			t.Fatal("open sections close and untouched ones are skipped")
		}
		if err := e.enterSection(f.id(1), at(61)); !errors.Is(err, errAttemptEnded) {
			t.Fatalf("ended attempts accept nothing, got %v", err)
		}
	}
}

func TestDisconnectWithoutResume(t *testing.T) {
	f := newFixture(1)
	f.cfg.Attempts.AllowResumeOnDisconnect = false
	e := f.build()
	must(t, e.enterSection(f.id(0), at(1)))
	e.touch(at(5))
	must(t, e.reconcile(at(6.5))) // within the 2-minute grace
	if !e.attempt.running() {
		t.Fatal("still within grace")
	}
	must(t, e.reconcile(at(30)))
	if e.attempt.Status != StatusAutoSubmitted || *e.attempt.EndedReason != EndDisconnected || !e.attempt.SubmittedAt.Equal(at(7)) {
		t.Fatalf("expected disconnect submit at minute 7, got %+v", e.attempt)
	}
}

func TestFreeNavigationLocksAndGates(t *testing.T) {
	f := newFixture(3)
	f.sections[0].LockOnComplete = true
	f.sections[1].UnlockAfterMinutes = ptr(20)
	f.sections[2].Prerequisites = []exams.Prerequisite{{SectionID: f.id(0), MinScorePercent: ptr(50.0)}}
	f.scores[f.id(0)] = 1 // 1 of 3 marks
	e := f.build()

	mustConflict(t, e.enterSection(f.id(1), at(5)))
	if a := e.access(e.state(f.id(1)), e.clockAt(at(5))); a.Reason != "unlocks_later" || !a.UnlocksAt.Equal(at(20)) {
		t.Fatalf("unexpected access %+v", a)
	}
	must(t, e.enterSection(f.id(0), at(5)))
	must(t, e.enterSection(f.id(1), at(21))) // leaving A seals it
	mustConflict(t, e.enterSection(f.id(0), at(22)))
	if a := e.access(e.state(f.id(2)), e.clockAt(at(22))); a.Reason != "score_gate_failed" {
		t.Fatalf("33%% should fail a 50%% gate, got %+v", a)
	}
}

func TestPauseStopsClocks(t *testing.T) {
	f := newFixture(1)
	f.cfg.Schedule.AllowCandidatePause = true
	f.cfg.Schedule.MaxPauseCount = 1
	f.cfg.Schedule.MaxTotalPauseMinutes = 10
	f.cfg.Schedule.PerSectionTiming = true
	f.limits = map[int]int{0: 30}
	e := f.build()
	must(t, e.enterSection(f.id(0), at(0)))
	must(t, e.pause(at(10)))
	mustConflict(t, e.navigate(1, true, at(11)))
	must(t, e.reconcile(at(25))) // pause budget ran out at 20
	if e.attempt.PausedAt != nil || e.attempt.PausedSeconds != 600 || !e.current().EnteredAt.Equal(at(20)) {
		t.Fatalf("pause should auto-end at minute 20: %+v", e.attempt)
	}
	mustConflict(t, e.pause(at(26))) // no pauses left
	if left, _ := sectionRemaining(e.current(), at(25)); left != 15*time.Minute {
		t.Fatalf("section clock should exclude the pause, %v left", left)
	}
	if c := e.clockAt(at(25)); !c.deadline.Equal(at(70)) {
		t.Fatalf("exam deadline should move by the pause, got %v", c.deadline)
	}
}

func TestQuestionNavigationModes(t *testing.T) {
	cases := []struct {
		mode    exams.QuestionNavigationMode
		revisit bool
		steps   []struct {
			to       int
			complete bool
			ok       bool
		}
	}{
		{exams.NavigationFreeJump, false, []struct {
			to       int
			complete bool
			ok       bool
		}{{2, false, true}, {1, false, true}, {2, false, false}, {0, false, false}}}, // unseen questions stay reachable; seen ones do not
		{exams.NavigationSequentialOnly, true, []struct {
			to       int
			complete bool
			ok       bool
		}{{2, true, false}, {1, false, true}, {0, true, false}}},
		{exams.NavigationMandatoryAnswer, true, []struct {
			to       int
			complete bool
			ok       bool
		}{{1, false, false}, {1, true, true}, {0, false, true}}},
	}
	for _, tc := range cases {
		f := newFixture(1)
		f.cfg.Navigation.QuestionNavigationMode = tc.mode
		f.cfg.Navigation.AllowRevisitingQuestions = tc.revisit
		e := f.build()
		must(t, e.enterSection(f.id(0), at(1)))
		for i, step := range tc.steps {
			// The question under the cursor counts as viewed.
			e.paper[f.id(0)][e.current().Cursor].FirstViewedAt = ptr(at(1))
			err := e.navigate(step.to, step.complete, at(2))
			if (err == nil) != step.ok {
				t.Fatalf("%s step %d (to %d): err=%v", tc.mode, i, step.to, err)
			}
		}
	}
}

func TestPaperIsDeterministicAndRespectsPins(t *testing.T) {
	opts := []optionSource{{ID: uuid.New()}, {ID: uuid.New()}, {ID: uuid.New()}, {ID: uuid.New(), Pinned: true}}
	var qs []paperSource
	for range 10 {
		qs = append(qs, paperSource{ID: uuid.New(), Points: 2, Options: opts})
	}
	src := []paperSection{{
		ID:        uuid.New(),
		Rules:     exams.EffectiveRules{ShuffleQuestions: true, ShuffleOptions: true, QuestionPickCount: ptr(4)},
		Questions: qs,
	}}
	_, p1 := buildPaper(42, src, false)
	_, p2 := buildPaper(42, src, false)
	if len(p1) != 4 || !slices.EqualFunc(p1, p2, func(a, b PaperQuestion) bool {
		return a.QuestionID == b.QuestionID && slices.Equal(a.OptionOrder, b.OptionOrder)
	}) {
		t.Fatal("the same seed must build the same paper")
	}
	for _, q := range p1 {
		if q.OptionOrder[3] != opts[3].ID {
			t.Fatal("pinned option moved")
		}
	}

	src[0].Rules = exams.EffectiveRules{QuestionPickCount: ptr(4)}
	_, p3 := buildPaper(7, src, false)
	idx := func(id uuid.UUID) int { return slices.IndexFunc(qs, func(q paperSource) bool { return q.ID == id }) }
	for i := 1; i < len(p3); i++ {
		if idx(p3[i-1].QuestionID) > idx(p3[i].QuestionID) {
			t.Fatal("without shuffling, drawn questions keep authored order")
		}
	}
}

func TestGrading(t *testing.T) {
	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mcq := &questions.Question{Type: questions.TypeMultipleChoice, Points: 4, Options: []questions.Option{
		{ID: a, IsCorrect: true, Weight: 1}, {ID: b, Weight: 1},
	}}
	neg := scoringPolicy{NegativeMarking: true, PenaltyPercent: 25, MultiSelectMode: "partial_with_penalty"}
	if g := grade(mcq, 4, &Response{SelectedOptionID: &a}, neg); g.Score != 4 || !*g.Correct {
		t.Fatalf("correct mcq: %+v", g)
	}
	if g := grade(mcq, 4, &Response{SelectedOptionID: &b}, neg); g.Score != -1 {
		t.Fatalf("wrong mcq with 25%% penalty: %+v", g)
	}
	if g := grade(mcq, 4, &Response{SelectedOptionID: &b}, scoringPolicy{}); g.Score != 0 {
		t.Fatalf("no negative marking: %+v", g)
	}
	if g := grade(mcq, 4, nil, neg); g.Score != 0 || g.Correct != nil || !g.Auto {
		t.Fatalf("unanswered: %+v", g)
	}

	ms := &questions.Question{Type: questions.TypeMultiSelect, Options: []questions.Option{
		{ID: a, IsCorrect: true, Weight: 1}, {ID: b, IsCorrect: true, Weight: 3}, {ID: c, Weight: 1}, {ID: d, Weight: 1},
	}}
	for _, tc := range []struct {
		mode string
		pick []uuid.UUID
		want float64
	}{
		{"all_or_nothing", []uuid.UUID{a, b}, 8},
		{"all_or_nothing", []uuid.UUID{a}, -2},
		{"partial_no_penalty", []uuid.UUID{b}, 6},
		{"partial_no_penalty", []uuid.UUID{a, b, c}, 0},
		{"partial_with_penalty", []uuid.UUID{a, b, c}, 4},
		{"partial_with_penalty", []uuid.UUID{c, d}, -2},
	} {
		p := neg
		p.MultiSelectMode = exams.MultiSelectScoringMode(tc.mode)
		if g := grade(ms, 8, &Response{SelectedOptionIDs: tc.pick}, p); g.Score != tc.want {
			t.Errorf("%s %v: got %v want %v", tc.mode, len(tc.pick), g.Score, tc.want)
		}
	}

	blank := &questions.Question{Type: questions.TypeBlankFill, BlankFill: &questions.BlankFillSpec{
		CorrectAnswer: "New Delhi", AlternateAnswers: []string{"Delhi"},
	}}
	for answer, want := range map[string]bool{"  new   delhi ": true, "DELHI": true, "Mumbai": false, " ": false} {
		if g := grade(blank, 1, &Response{TextResponse: &answer}, scoringPolicy{}); *g.Correct != want {
			t.Errorf("blank %q: got %v", answer, *g.Correct)
		}
	}
	essay := &questions.Question{Type: questions.TypeEssay}
	if g := grade(essay, 10, &Response{TextResponse: ptr("text")}, neg); g.Auto {
		t.Fatal("essays need manual grading")
	}
}

func TestAvailability(t *testing.T) {
	cfg := testConfig()
	cfg.Schedule.ScheduleType = exams.ScheduleFixedWindow
	cfg.Schedule.StartTime, cfg.Schedule.EndTime = ptr(at(0)), ptr(at(120))
	cfg.Schedule.LateJoinGraceMinutes = 15
	c := &candidacy{ExamStatus: exams.StatusPublished, Config: cfg, TimeMultiplier: 1.5}

	for _, tc := range []struct {
		now    float64
		state  string
		launch bool
		start  bool
	}{
		{-30, "upcoming", false, false},
		{-10, "upcoming", true, false},
		{5, "open", true, true},
		{15, "closed", false, false},
	} {
		av := availability(c, at(tc.now))
		if av.State != tc.state || av.CanLaunch != tc.launch || av.CanStart != tc.start {
			t.Errorf("at %v: %+v", tc.now, av)
		}
	}
	if p := planTimer(c, at(5)); !p.Origin.Equal(at(0)) || p.LimitSeconds != 5400 || !p.WindowEnd.Equal(at(120)) {
		t.Fatalf("fixed window timer plan wrong: %+v", p)
	}

	c.AttemptsUsed = 1
	if av := availability(c, at(5)); av.Reason != "no_attempts_left" {
		t.Fatalf("expected no attempts left, got %+v", av)
	}
	c.InProgressID = ptr(uuid.New())
	if av := availability(c, at(60)); av.State != "in_progress" || !av.CanLaunch {
		t.Fatalf("a running attempt can always be resumed: %+v", av)
	}
}
