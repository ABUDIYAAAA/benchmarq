package assignments

import (
	"time"

	"github.com/google/uuid"
)

// Assignment gives an organization member access to an exam.
type Assignment struct {
	ID                uuid.UUID  `json:"id"`
	ExamID            uuid.UUID  `json:"exam_id"`
	UserID            uuid.UUID  `json:"user_id"`
	Email             string     `json:"email"`
	FirstName         string     `json:"first_name"`
	LastName          *string    `json:"last_name,omitempty"`
	AssignedByUserID  *uuid.UUID `json:"assigned_by_user_id,omitempty"`
	InviteEmailSentAt *time.Time `json:"invite_email_sent_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// examRef is the part of an exam that assignment rules and emails need.
type examRef struct {
	Title                string
	Status               string
	ScheduleType         string
	StartTime            *time.Time
	TotalDurationMinutes int
}
