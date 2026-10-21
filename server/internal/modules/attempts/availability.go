package attempts

import (
	"math"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/google/uuid"
)

// earlyLaunchWindow is how long before a window opens the desktop app may
// be launched, so candidates can pass device checks and read instructions.
const earlyLaunchWindow = 15 * time.Minute

// Availability is when and whether a candidate may take an exam.
type Availability struct {
	// State is upcoming, open, in_progress or closed.
	State string `json:"state"`
	// Reason explains a closed or upcoming state, e.g. late_join_closed.
	Reason string `json:"reason,omitempty"`
	// CanLaunch reports whether the desktop app may be launched now.
	CanLaunch bool `json:"can_launch"`
	// CanStart reports whether a new attempt may start now.
	CanStart bool `json:"can_start"`

	OpensAt     *time.Time `json:"opens_at,omitempty"`
	StartBy     *time.Time `json:"start_by,omitempty"`
	ClosesAt    *time.Time `json:"closes_at,omitempty"`
	AvailableAt *time.Time `json:"available_at,omitempty"` // end of a retake cooldown

	DurationMinutes     int        `json:"duration_minutes"`
	AttemptsUsed        int        `json:"attempts_used"`
	MaxAttempts         int        `json:"max_attempts"`
	InProgressAttemptID *uuid.UUID `json:"in_progress_attempt_id,omitempty"`
}

// candidacy is everything availability depends on for one candidate.
type candidacy struct {
	ExamStatus     exams.ExamStatus
	Config         *exams.ExamConfig
	StartOverride  *time.Time
	EndOverride    *time.Time
	TimeMultiplier float64
	AttemptsUsed   int // attempts that have ended
	LastEndedAt    *time.Time
	InProgressID   *uuid.UUID
}

func (c *candidacy) window() (start, end *time.Time) {
	start, end = c.Config.Schedule.StartTime, c.Config.Schedule.EndTime
	if c.StartOverride != nil {
		start = c.StartOverride
	}
	if c.EndOverride != nil {
		end = c.EndOverride
	}
	return start, end
}

func (c *candidacy) limitSeconds() int {
	m := c.TimeMultiplier
	if m <= 0 {
		m = 1
	}
	return int(math.Round(float64(c.Config.Schedule.TotalDurationMinutes) * 60 * m))
}

// scaled applies the candidate's time multiplier to a duration in minutes.
func (c *candidacy) scaled(minutes int) int {
	m := c.TimeMultiplier
	if m <= 0 {
		m = 1
	}
	return int(math.Round(float64(minutes) * 60 * m))
}

func availability(c *candidacy, now time.Time) Availability {
	sc := c.Config.Schedule
	av := Availability{
		DurationMinutes: c.limitSeconds() / 60,
		AttemptsUsed:    c.AttemptsUsed,
		MaxAttempts:     c.Config.Attempts.MaxAttempts,
	}
	if c.InProgressID != nil {
		av.State, av.CanLaunch, av.InProgressAttemptID = "in_progress", true, c.InProgressID
		return av
	}
	closedWith := func(reason string) Availability {
		av.State, av.Reason = "closed", reason
		return av
	}
	if c.ExamStatus != exams.StatusPublished {
		return closedWith("not_published")
	}

	start, end := c.window()
	switch sc.ScheduleType {
	case exams.ScheduleFixedWindow:
		if start == nil {
			return closedWith("not_scheduled")
		}
		startBy := start.Add(time.Duration(sc.LateJoinGraceMinutes) * time.Minute)
		if end != nil && end.Before(startBy) {
			startBy = *end
		}
		av.OpensAt, av.StartBy, av.ClosesAt = start, &startBy, end
	case exams.ScheduleFlexibleWindow:
		if start == nil || end == nil {
			return closedWith("not_scheduled")
		}
		av.OpensAt, av.StartBy, av.ClosesAt = start, end, end
	}

	if av.MaxAttempts > 0 && c.AttemptsUsed >= av.MaxAttempts {
		return closedWith("no_attempts_left")
	}
	if av.StartBy != nil && !now.Before(*av.StartBy) {
		if sc.ScheduleType == exams.ScheduleFixedWindow {
			return closedWith("late_join_closed")
		}
		return closedWith("window_closed")
	}
	if c.LastEndedAt != nil && c.Config.Attempts.AttemptCooldownHours > 0 {
		ready := c.LastEndedAt.Add(time.Duration(c.Config.Attempts.AttemptCooldownHours) * time.Hour)
		if now.Before(ready) {
			av.AvailableAt = &ready
			return closedWith("cooldown")
		}
	}
	if av.OpensAt != nil && now.Before(*av.OpensAt) {
		av.State, av.Reason = "upcoming", "not_open_yet"
		av.CanLaunch = !now.Before(av.OpensAt.Add(-earlyLaunchWindow))
		return av
	}
	av.State, av.CanLaunch, av.CanStart = "open", true, true
	return av
}

// timerPlan is the frozen timing of a new attempt.
type timerPlan struct {
	Origin       time.Time
	LimitSeconds int
	WindowEnd    *time.Time
}

// planTimer fixes the attempt clock at start. Fixed-window exams run from the
// window start, so late joiners get less time; other schedules run from now.
// Windows cap the deadline.
func planTimer(c *candidacy, now time.Time) timerPlan {
	start, end := c.window()
	p := timerPlan{Origin: now, LimitSeconds: c.limitSeconds()}
	switch c.Config.Schedule.ScheduleType {
	case exams.ScheduleFixedWindow:
		p.Origin = *start
		p.WindowEnd = end
	case exams.ScheduleFlexibleWindow:
		p.WindowEnd = end
	}
	return p
}
