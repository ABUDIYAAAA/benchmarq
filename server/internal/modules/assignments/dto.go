package assignments

import "github.com/google/uuid"

type AssignRequest struct {
	// UserIDs are organization members to assign, at most 500 per request.
	UserIDs []uuid.UUID `json:"user_ids" validate:"required,min=1,max=500,unique"`
	// SendEmail emails every listed user an exam invitation, including users
	// who were already assigned. Only allowed once the exam is published.
	SendEmail bool `json:"send_email"`
}

type AssignResult struct {
	// Assignments holds one entry per requested user, new or existing.
	Assignments []Assignment `json:"assignments"`
	Created     int          `json:"created"`
	Emailed     int          `json:"emailed"`
}
