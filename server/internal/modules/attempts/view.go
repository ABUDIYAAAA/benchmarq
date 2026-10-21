package attempts

import (
	"context"
	"math"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/questions"
	"github.com/google/uuid"
)

// Rendering of candidate views. Question content is copied field by field
// from the authoring model so answer keys, explanations, option weights,
// rubrics and sample answers can never reach the candidate.

func (s *service) runtimeState(ctx context.Context, rt *runtime, now time.Time) (*RuntimeState, error) {
	brief, err := s.examBrief(ctx, rt.bp)
	if err != nil {
		return nil, err
	}
	view, err := s.attemptView(ctx, rt, now)
	if err != nil {
		return nil, err
	}
	return &RuntimeState{ServerTime: now, Exam: *brief, Rules: rulesFrom(rt.bp.cfg), Attempt: view}, nil
}

func (s *service) attemptView(ctx context.Context, rt *runtime, now time.Time) (*AttemptView, error) {
	e, a := rt.eng, rt.attempt()
	violations, err := s.repo.CountViolations(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	v := &AttemptView{
		ID: a.ID, AttemptNumber: a.AttemptNumber, Status: a.Status, StartedAt: a.StartedAt,
		SubmittedAt: a.SubmittedAt, EndedReason: a.EndedReason, Violations: violations,
		Sections: make([]SectionView, 0, len(e.states)),
	}

	running := a.running()
	c := e.clockAt(now)
	if running {
		v.Clock = clockView(rt, c)
	}
	for _, st := range e.states {
		info := e.sections[st.SectionID]
		qs := e.paper[st.SectionID]
		sv := SectionView{
			ID: st.SectionID, Name: info.Section.Name, Type: info.Section.SectionType,
			Instructions: info.Section.Instructions, Position: st.Position, Status: st.Status,
			IsOpen:        a.CurrentSectionID != nil && *a.CurrentSectionID == st.SectionID,
			QuestionCount: len(qs), TimeLimitSeconds: st.TimeLimitSeconds,
		}
		for _, pq := range qs {
			if e.responses[pq.QuestionID].answered() {
				sv.AnsweredCount++
			}
		}
		v.Answered += sv.AnsweredCount
		v.Total += sv.QuestionCount
		if left, timed := sectionRemaining(st, now); timed {
			sv.RemainingSeconds = ptrTo(ceilSeconds(left))
		}
		if running && !sv.IsOpen {
			acc := e.access(st, c)
			sv.CanEnter, sv.LockReason, sv.UnlocksAt = acc.Open && !c.paused, acc.Reason, acc.UnlocksAt
		}
		v.Sections = append(v.Sections, sv)
	}

	if cur := e.current(); running && cur != nil {
		qs := e.paper[cur.SectionID]
		open := &OpenSectionView{SectionID: cur.SectionID, Cursor: cur.Cursor, Slots: make([]SlotView, len(qs))}
		curDone := cur.Cursor < len(qs) && e.responses[qs[cur.Cursor].QuestionID].answered()
		for i, pq := range qs {
			r := e.responses[pq.QuestionID]
			open.Slots[i] = SlotView{
				Position: i, Viewed: pq.FirstViewedAt != nil, Answered: r.answered(), Flagged: r != nil && r.Flagged,
				Reachable: !c.paused && e.checkMove(cur, qs, i, curDone) == nil,
			}
		}
		v.Current = open
	}
	return v, nil
}

func clockView(rt *runtime, c clock) *ClockView {
	e, a := rt.eng, rt.attempt()
	sc := rt.bp.cfg.Schedule
	cv := &ClockView{
		ServerTime:       c.now,
		DeadlineAt:       c.deadline,
		RemainingSeconds: ceilSeconds(c.remaining()),
		Paused:           c.paused,
		PauseSecondsLeft: ceilSeconds(c.pauseLeft),
		CanSubmitAt:      a.StartedAt.Add(time.Duration(sc.MinTimeBeforeSubmissionMinutes)*time.Minute + c.pausedTotal),
	}
	if sc.AllowCandidatePause {
		cv.PausesLeft = max(sc.MaxPauseCount-a.PauseCount, 0)
	}
	if cur := e.current(); cur != nil {
		if left, timed := sectionRemaining(cur, c.now); timed {
			end := c.now.Add(left)
			if end.After(c.deadline) {
				end, left = c.deadline, c.remaining()
			}
			cv.Section = &TimerView{SectionID: cur.SectionID, DeadlineAt: end, RemainingSeconds: ceilSeconds(left)}
		}
	}
	return cv
}

func (s *service) questionView(rt *runtime, pq *PaperQuestion, q *questions.Question, now time.Time) *QuestionView {
	qs := rt.eng.paper[pq.SectionID]
	v := &QuestionView{
		SectionID: pq.SectionID, Position: pq.Position, IsLast: pq.Position == len(qs)-1,
		QuestionID: q.ID, Type: string(q.Type), Prompt: q.Prompt, Points: pq.Points,
		Clock: clockView(rt, rt.eng.clockAt(now)),
	}
	if sc := rt.bp.cfg.Scoring; sc.NegativeMarkingEnabled && q.Type != questions.TypeShortAnswer && q.Type != questions.TypeEssay {
		penalty := q.NegativePoints
		if penalty == 0 {
			penalty = pq.Points * sc.NegativeMarkingPenaltyPercent / 100
		}
		if penalty > 0 {
			v.Penalty = ptrTo(round2(penalty))
		}
	}

	byID := make(map[uuid.UUID]string, len(q.Options))
	for _, o := range q.Options {
		byID[o.ID] = o.Content
	}
	for _, id := range pq.OptionOrder {
		if content, ok := byID[id]; ok {
			v.Options = append(v.Options, OptionView{ID: id, Content: content})
		}
	}

	switch {
	case q.MultiSelect != nil:
		v.MultiSelect = &MultiSelectView{MinChoices: q.MultiSelect.MinChoicesRequired, MaxChoices: q.MultiSelect.MaxChoicesAllowed}
	case q.ShortAnswer != nil:
		v.ShortAnswer = &TextLimitsView{MaxLength: q.ShortAnswer.MaxLength, MaxWords: q.ShortAnswer.MaxWords}
	case q.Essay != nil:
		v.Essay = &EssayView{MinWords: q.Essay.MinWords, MaxWords: q.Essay.MaxWords, MaxLength: q.Essay.MaxLength, AllowRichText: q.Essay.AllowRichText}
	case q.Type == questions.TypeBlankFill:
		v.ShortAnswer = &TextLimitsView{MaxLength: ptrTo(blankFillMaxLength)}
	}

	if r := rt.eng.responses[pq.QuestionID]; r != nil {
		v.Response = responseView(r)
	}
	return v
}

func responseView(r *Response) *ResponseView {
	return &ResponseView{
		SelectedOptionID: r.SelectedOptionID, SelectedOptionIDs: r.SelectedOptionIDs,
		Text: r.TextResponse, Boolean: r.BooleanResponse, Flagged: r.Flagged, SavedAt: r.UpdatedAt,
	}
}

// ceilSeconds rounds up, so a countdown never shows 0 while time remains.
func ceilSeconds(d time.Duration) int {
	return int(math.Ceil(d.Seconds()))
}
