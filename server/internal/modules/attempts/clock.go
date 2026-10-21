package attempts

import "time"

// The attempt clock. Every value is derived from fields frozen on the
// attempt and the current time, so the server never trusts a client timer
// and any instance can recompute the same deadline.

func secs(n int) time.Duration { return time.Duration(n) * time.Second }

// clock is an attempt's timer state at one instant.
type clock struct {
	now time.Time
	// deadline is when the exam ends. While paused it assumes the pause ends
	// now, so it moves forward until the pause budget runs out.
	deadline time.Time
	// pausedTotal is pause time used so far, including a running pause.
	pausedTotal time.Duration
	paused      bool
	pauseLeft   time.Duration
}

// computeClock evaluates the attempt's timers at now. pauseBudget is the
// total pause time the exam allows.
func computeClock(a *Attempt, pauseBudget time.Duration, now time.Time) clock {
	pausedTotal := secs(a.PausedSeconds)
	paused := false
	if a.PausedAt != nil {
		left := max(pauseBudget-pausedTotal, 0)
		run := min(max(now.Sub(*a.PausedAt), 0), left)
		pausedTotal += run
		paused = run < left
	}

	deadline := a.TimerOriginAt.Add(secs(a.LimitSeconds) + pausedTotal)
	if a.WindowEndAt != nil && a.WindowEndAt.Before(deadline) {
		deadline = *a.WindowEndAt
	}
	return clock{
		now:         now,
		deadline:    deadline,
		pausedTotal: pausedTotal,
		paused:      paused,
		pauseLeft:   max(pauseBudget-pausedTotal, 0),
	}
}

func (c clock) expired() bool { return !c.now.Before(c.deadline) }

func (c clock) remaining() time.Duration { return max(c.deadline.Sub(c.now), 0) }

// activeElapsed is unpaused time since the candidate started.
func (c clock) activeElapsed(a *Attempt) time.Duration {
	return max(c.now.Sub(a.StartedAt)-c.pausedTotal, 0)
}

// latestDeadline is the latest the attempt can run if every remaining second
// of pause budget is used. The expiry sweeper indexes on it.
func latestDeadline(a *Attempt, pauseBudget time.Duration) time.Time {
	extra := secs(a.PausedSeconds)
	if a.PausedAt != nil {
		extra = max(pauseBudget, extra)
	}
	deadline := a.TimerOriginAt.Add(secs(a.LimitSeconds) + extra)
	if a.WindowEndAt != nil && a.WindowEndAt.Before(deadline) {
		deadline = *a.WindowEndAt
	}
	return deadline
}

// sectionRemaining is the time left on a section's own timer. ok is false
// for sections without one.
func sectionRemaining(s *SectionState, now time.Time) (left time.Duration, ok bool) {
	if s.TimeLimitSeconds == nil {
		return 0, false
	}
	spent := secs(s.TimeSpentSeconds)
	if s.EnteredAt != nil {
		spent += max(now.Sub(*s.EnteredAt), 0)
	}
	return max(secs(*s.TimeLimitSeconds)-spent, 0), true
}

// sectionDeadline is when the open section's timer runs out. ok is false if
// the section has no timer or is not running.
func sectionDeadline(s *SectionState) (time.Time, bool) {
	if s.TimeLimitSeconds == nil || s.EnteredAt == nil {
		return time.Time{}, false
	}
	return s.EnteredAt.Add(secs(*s.TimeLimitSeconds - s.TimeSpentSeconds)), true
}

// foldVisit adds the running visit to the section's spent time, as of at.
func foldVisit(s *SectionState, at time.Time) {
	if s.EnteredAt == nil {
		return
	}
	spent := s.TimeSpentSeconds + int(max(at.Sub(*s.EnteredAt), 0)/time.Second)
	if s.TimeLimitSeconds != nil {
		spent = min(spent, *s.TimeLimitSeconds)
	}
	s.TimeSpentSeconds = spent
	s.EnteredAt = nil
}
