package attempts

import (
	"fmt"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/google/uuid"
)

// The engine is the attempt state machine. It works on an attempt loaded in
// memory, with no I/O except the injected section scorer, and decides what
// the candidate may do and see. Callers load state, call reconcile, apply one
// action and persist the result, all under the attempt's row lock.

var (
	errAttemptEnded   = apperr.Conflict("This attempt has ended")
	errAttemptPaused  = apperr.Conflict("The exam is paused; resume it first")
	errNoOpenSection  = apperr.Conflict("Open a section first")
	errSectionUnknown = apperr.NotFound("Section not found in this attempt")
)

// sectionInfo is a section's authored definition and effective rules.
type sectionInfo struct {
	Section exams.Section
	Rules   exams.EffectiveRules
}

// sectionScorer grades a section's served objective questions. awarded is
// nil for sections that need manual grading.
type sectionScorer func(sectionID uuid.UUID) (awarded *float64, maxScore float64, err error)

type engine struct {
	cfg       *exams.ExamConfig
	attempt   *Attempt
	sections  map[uuid.UUID]sectionInfo
	states    []*SectionState // candidate order
	paper     map[uuid.UUID][]*PaperQuestion
	responses map[uuid.UUID]*Response
	score     sectionScorer
	// onEnd, if set, runs as soon as the attempt ends, so grading is done
	// before anything about the ended attempt is shown.
	onEnd func() error
}

func newEngine(cfg *exams.ExamConfig, a *Attempt, sections map[uuid.UUID]sectionInfo, states []SectionState,
	paper []PaperQuestion, responses []Response, score sectionScorer) *engine {
	e := &engine{
		cfg:       cfg,
		attempt:   a,
		sections:  sections,
		states:    make([]*SectionState, len(states)),
		paper:     make(map[uuid.UUID][]*PaperQuestion),
		responses: make(map[uuid.UUID]*Response, len(responses)),
		score:     score,
	}
	for i := range states {
		e.states[i] = &states[i]
	}
	for i := range paper {
		q := &paper[i]
		e.paper[q.SectionID] = append(e.paper[q.SectionID], q)
	}
	for i := range responses {
		e.responses[responses[i].QuestionID] = &responses[i]
	}
	return e
}

func (e *engine) pauseBudget() time.Duration {
	if !e.cfg.Schedule.AllowCandidatePause {
		return 0
	}
	return time.Duration(e.cfg.Schedule.MaxTotalPauseMinutes) * time.Minute
}

func (e *engine) clockAt(now time.Time) clock { return computeClock(e.attempt, e.pauseBudget(), now) }

func (e *engine) mode() exams.SectionProgressionMode { return e.cfg.Navigation.SectionProgressionMode }

func (e *engine) linear() bool { return e.mode() != exams.ProgressionFreeNavigation }

func (e *engine) state(id uuid.UUID) *SectionState {
	for _, s := range e.states {
		if s.SectionID == id {
			return s
		}
	}
	return nil
}

func (e *engine) current() *SectionState {
	if e.attempt.CurrentSectionID == nil {
		return nil
	}
	return e.state(*e.attempt.CurrentSectionID)
}

// ---------------------------------------------------------------------------
// Time
// ---------------------------------------------------------------------------

// reconcile applies everything that happened by itself since the attempt was
// last touched, in the order it happened: an exhausted pause ends, timed
// sections close (and in timed_sequential mode the next one opens), a
// candidate who lost contact past the grace period is submitted, and the
// exam expires. Each transition is applied at the instant it happened, not
// at now, so the outcome does not depend on when the server looks.
func (e *engine) reconcile(now time.Time) error {
	if !e.attempt.running() {
		return nil
	}
	e.settlePause(now)
	for {
		c := e.clockAt(now)

		var sectionEnd time.Time
		sectionDue := false
		if cur := e.current(); cur != nil && !c.paused {
			if end, ok := sectionDeadline(cur); ok && !now.Before(end) && end.Before(c.deadline) {
				sectionEnd, sectionDue = end, true
			}
		}
		lostAt, lost := e.disconnectedAt(c)
		lost = lost && !now.Before(lostAt) && lostAt.Before(c.deadline)

		switch {
		case sectionDue && (!lost || sectionEnd.Before(lostAt)):
			cur := e.current()
			if err := e.closeSection(cur, sectionEnd); err != nil {
				return err
			}
			if e.mode() == exams.ProgressionTimedSequential {
				if err := e.advance(cur, sectionEnd); err != nil {
					return err
				}
			}
			continue
		case lost:
			return e.end(lostAt, EndDisconnected, StatusAutoSubmitted)
		case c.expired():
			status := StatusAutoSubmitted
			if !e.cfg.Schedule.AutoSubmitOnExpiry {
				status = StatusAbandoned
			}
			return e.end(c.deadline, EndTimeExpired, status)
		}
		return nil
	}
}

// disconnectedAt is when a silent candidate loses the attempt: the last sign
// of life plus the reconnection grace. It only applies when the exam forbids
// resuming after a disconnect, and never while paused.
func (e *engine) disconnectedAt(c clock) (time.Time, bool) {
	a := e.attempt
	if e.cfg.Attempts.AllowResumeOnDisconnect || c.paused || a.LastHeartbeatAt == nil {
		return time.Time{}, false
	}
	return a.LastHeartbeatAt.Add(secs(e.cfg.Schedule.ReconnectionGraceSeconds)), true
}

// touch records a sign of life from the candidate.
func (e *engine) touch(now time.Time) {
	if e.attempt.LastHeartbeatAt == nil || e.attempt.LastHeartbeatAt.Before(now) {
		e.attempt.LastHeartbeatAt = &now
	}
}

// settlePause ends a pause whose budget has run out, at the moment it ran out.
func (e *engine) settlePause(now time.Time) {
	a := e.attempt
	if a.PausedAt == nil {
		return
	}
	left := max(e.pauseBudget()-secs(a.PausedSeconds), 0)
	if now.Sub(*a.PausedAt) < left {
		return
	}
	resumedAt := a.PausedAt.Add(left)
	a.PausedSeconds += int(left / time.Second)
	a.PausedAt = nil
	if cur := e.current(); cur != nil {
		cur.EnteredAt = &resumedAt
	}
	// Contact is measured from the end of the pause, not from before it.
	e.touch(resumedAt)
}

// requireActive fails unless the candidate can act right now.
func (e *engine) requireActive(now time.Time) error {
	if !e.attempt.running() {
		return errAttemptEnded
	}
	if e.clockAt(now).paused {
		return errAttemptPaused
	}
	return nil
}

func (e *engine) pause(now time.Time) error {
	if err := e.requireActive(now); err != nil {
		return err
	}
	sc := e.cfg.Schedule
	switch {
	case !sc.AllowCandidatePause:
		return apperr.Forbidden("This exam does not allow pausing")
	case e.attempt.PauseCount >= sc.MaxPauseCount:
		return apperr.Conflict("You have used all of your pauses")
	case e.clockAt(now).pauseLeft <= 0:
		return apperr.Conflict("You have used all of your pause time")
	}
	if cur := e.current(); cur != nil {
		foldVisit(cur, now)
	}
	e.attempt.PausedAt = &now
	e.attempt.PauseCount++
	return nil
}

func (e *engine) resume(now time.Time) error {
	if !e.attempt.running() {
		return errAttemptEnded
	}
	if e.attempt.PausedAt == nil {
		return apperr.Conflict("The exam is not paused")
	}
	e.attempt.PausedSeconds += int(max(now.Sub(*e.attempt.PausedAt), 0) / time.Second)
	e.attempt.PausedAt = nil
	if cur := e.current(); cur != nil {
		cur.EnteredAt = &now
	}
	e.touch(now)
	return nil
}

// ---------------------------------------------------------------------------
// Sections
// ---------------------------------------------------------------------------

// access describes whether a section can be opened.
type access struct {
	Open      bool
	Reason    string // why not, when !Open
	UnlocksAt *time.Time
}

func closed(reason string) access { return access{Reason: reason} }

func (e *engine) access(st *SectionState, c clock) access {
	info := e.sections[st.SectionID]
	if left, timed := sectionRemaining(st, c.now); timed && left <= 0 {
		return closed("time_up")
	}
	if st.Status == SectionCompleted || st.Status == SectionSkipped {
		if info.Rules.LockOnComplete || st.Status == SectionSkipped {
			return closed("completed")
		}
	}
	for _, p := range e.prerequisites(st) {
		ps := e.state(p.SectionID)
		if ps == nil {
			continue
		}
		if ps.Status != SectionCompleted {
			return closed("prerequisite")
		}
		if p.MinScorePercent != nil && !meetsScore(ps, *p.MinScorePercent) {
			return closed("score_gate_failed")
		}
	}
	if u := info.Rules.UnlockAfterMinutes; u != nil {
		wait := time.Duration(*u)*time.Minute - c.activeElapsed(e.attempt)
		if wait > 0 {
			a := closed("unlocks_later")
			if !c.paused {
				at := c.now.Add(wait)
				a.UnlocksAt = &at
			}
			return a
		}
	}
	return access{Open: true}
}

// prerequisites returns the section's effective prerequisites. With shuffled
// sections in a linear mode, the implied "previous section" follows the
// candidate's own order.
func (e *engine) prerequisites(st *SectionState) []exams.Prerequisite {
	prereqs := e.sections[st.SectionID].Rules.Prerequisites
	if e.linear() && e.cfg.Randomization.ShuffleSections && st.Position > 0 {
		prev := e.states[st.Position-1].SectionID
		prereqs = append([]exams.Prerequisite{{SectionID: prev}}, prereqs...)
	}
	return prereqs
}

func meetsScore(st *SectionState, minPercent float64) bool {
	if st.MaxScore == nil || *st.MaxScore == 0 {
		return true
	}
	if st.ScoreAwarded == nil {
		return false
	}
	return *st.ScoreAwarded / *st.MaxScore * 100 >= minPercent
}

// enterSection opens a section. Leaving the open section seals it when its
// rules lock completed sections; otherwise it can be returned to.
func (e *engine) enterSection(id uuid.UUID, now time.Time) error {
	if err := e.requireActive(now); err != nil {
		return err
	}
	st := e.state(id)
	if st == nil {
		return errSectionUnknown
	}
	cur := e.current()
	if cur == st {
		return nil
	}
	if a := e.access(st, e.clockAt(now)); !a.Open {
		return apperr.Conflict(lockMessage(a.Reason))
	}
	if cur != nil {
		if e.sections[cur.SectionID].Rules.LockOnComplete {
			if err := e.closeSection(cur, now); err != nil {
				return err
			}
		} else {
			foldVisit(cur, now)
		}
	}
	e.open(st, now)
	return nil
}

func (e *engine) open(st *SectionState, at time.Time) {
	if st.Status == SectionNotStarted {
		st.Status = SectionInProgress
	}
	if st.UnlockedAt == nil {
		st.UnlockedAt = &at
	}
	st.EnteredAt = &at
	e.attempt.CurrentSectionID = &st.SectionID
}

// finishSection completes the open section. In linear modes the next section
// opens straight away.
func (e *engine) finishSection(id uuid.UUID, now time.Time) error {
	if err := e.requireActive(now); err != nil {
		return err
	}
	st := e.state(id)
	if st == nil {
		return errSectionUnknown
	}
	if e.current() != st {
		return apperr.Conflict("Only the open section can be finished")
	}
	if err := e.closeSection(st, now); err != nil {
		return err
	}
	if e.linear() {
		return e.advance(st, now)
	}
	return nil
}

// advance opens the first section after from, in the candidate's order,
// that can be opened at the given instant.
func (e *engine) advance(from *SectionState, at time.Time) error {
	c := e.clockAt(at)
	for _, st := range e.states[from.Position+1:] {
		if e.access(st, c).Open {
			e.open(st, at)
			return nil
		}
	}
	return nil
}

// closeSection ends the visit to a section, marks it completed and scores
// it, so score-gated sections can unlock. A revisited section is re-scored.
func (e *engine) closeSection(st *SectionState, at time.Time) error {
	foldVisit(st, at)
	st.Status = SectionCompleted
	st.CompletedAt = &at
	if e.attempt.CurrentSectionID != nil && *e.attempt.CurrentSectionID == st.SectionID {
		e.attempt.CurrentSectionID = nil
	}
	awarded, maxScore, err := e.score(st.SectionID)
	if err != nil {
		return err
	}
	st.ScoreAwarded, st.MaxScore = awarded, &maxScore
	return nil
}

// end stops the attempt at the given instant.
func (e *engine) end(at time.Time, reason EndReason, status Status) error {
	a := e.attempt
	if a.PausedAt != nil {
		a.PausedSeconds += int(max(at.Sub(*a.PausedAt), 0) / time.Second)
		a.PausedAt = nil
	}
	for _, st := range e.states {
		switch {
		case st.Status == SectionNotStarted:
			st.Status = SectionSkipped
		case st.Status == SectionInProgress || st.EnteredAt != nil:
			if err := e.closeSection(st, at); err != nil {
				return err
			}
		}
	}
	a.CurrentSectionID = nil
	a.Status = status
	a.SubmittedAt = &at
	a.EndedReason = &reason
	if e.onEnd != nil {
		return e.onEnd()
	}
	return nil
}

func lockMessage(reason string) string {
	switch reason {
	case "time_up":
		return "This section's time is up"
	case "completed":
		return "This section is closed"
	case "prerequisite":
		return "Complete the earlier sections first"
	case "score_gate_failed":
		return "This section requires a higher score in an earlier section"
	case "unlocks_later":
		return "This section is not open yet"
	default:
		return "This section is not available"
	}
}

// ---------------------------------------------------------------------------
// Questions
// ---------------------------------------------------------------------------

// openQuestions returns the open section's questions in candidate order.
func (e *engine) openQuestions() (*SectionState, []*PaperQuestion, error) {
	cur := e.current()
	if cur == nil {
		return nil, nil, errNoOpenSection
	}
	return cur, e.paper[cur.SectionID], nil
}

// currentQuestion is the question under the cursor of the open section.
func (e *engine) currentQuestion(now time.Time) (*PaperQuestion, error) {
	if err := e.requireActive(now); err != nil {
		return nil, err
	}
	cur, qs, err := e.openQuestions()
	if err != nil {
		return nil, err
	}
	if cur.Cursor >= len(qs) {
		return nil, apperr.Conflict("This section has no questions")
	}
	return qs[cur.Cursor], nil
}

// navigate moves the cursor of the open section. currentComplete reports
// whether the question under the cursor has a complete answer, which
// mandatory_answer navigation requires before moving forward.
func (e *engine) navigate(target int, currentComplete bool, now time.Time) error {
	if err := e.requireActive(now); err != nil {
		return err
	}
	cur, qs, err := e.openQuestions()
	if err != nil {
		return err
	}
	if target < 0 || target >= len(qs) {
		return apperr.Validation(map[string]string{"position": fmt.Sprintf("position must be between 0 and %d", len(qs)-1)})
	}
	if err := e.checkMove(cur, qs, target, currentComplete); err != nil {
		return err
	}
	cur.Cursor = target
	return nil
}

// checkMove applies the question navigation mode:
//
//   - free_jump: any question; with revisiting off, a question once seen
//     cannot be returned to.
//   - sequential_only: one question forward at a time, never back.
//   - mandatory_answer: one question forward at a time once answered; back
//     only with revisiting on.
func (e *engine) checkMove(cur *SectionState, qs []*PaperQuestion, target int, currentComplete bool) error {
	if target == cur.Cursor {
		return nil
	}
	nav := e.cfg.Navigation
	revisit := nav.AllowRevisitingQuestions
	back := target < cur.Cursor
	switch nav.QuestionNavigationMode {
	case exams.NavigationFreeJump:
		if !revisit && qs[target].FirstViewedAt != nil {
			return apperr.Conflict("Questions cannot be revisited in this exam")
		}
	case exams.NavigationSequentialOnly:
		if back {
			return apperr.Conflict("Questions cannot be revisited in this exam")
		}
		if target != cur.Cursor+1 {
			return apperr.Conflict("Questions must be answered in order")
		}
	case exams.NavigationMandatoryAnswer:
		if back {
			if !revisit {
				return apperr.Conflict("Questions cannot be revisited in this exam")
			}
			return nil
		}
		if target != cur.Cursor+1 {
			return apperr.Conflict("Questions must be answered in order")
		}
		if !currentComplete {
			return apperr.Conflict("Answer this question before moving on")
		}
	}
	return nil
}
