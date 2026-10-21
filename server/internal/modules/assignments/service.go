package assignments

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/internal/mailer"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/pagination"
	"github.com/google/uuid"
)

var (
	errExamNotFound       = apperr.NotFound("Exam not found")
	errAssignmentNotFound = apperr.NotFound("Assignment not found")
	errExamClosed         = apperr.Conflict("Candidates can only be assigned to draft or published exams")
	errEmailNeedsPublish  = apperr.Validation(map[string]string{"send_email": "invitation emails can only be sent once the exam is published"})
)

// Options configures assignment invitation emails.
type Options struct {
	Mailer      mailer.Mailer // optional; send_email is a no-op when nil
	FrontendURL string
}

type Service interface {
	Assign(ctx context.Context, m authz.Membership, examID uuid.UUID, req AssignRequest) (*AssignResult, error)
	List(ctx context.Context, m authz.Membership, examID uuid.UUID, p pagination.Params) (pagination.Page[Assignment], error)
	Unassign(ctx context.Context, m authz.Membership, examID, assignmentID uuid.UUID) error
}

type service struct {
	repo   Repository
	tx     database.Transactor
	logger *slog.Logger
	opts   Options
}

func NewService(repo Repository, tx database.Transactor, logger *slog.Logger, opts Options) Service {
	return &service{repo: repo, tx: tx, logger: logger, opts: opts}
}

// Assign gives organization members access to an exam. It is idempotent:
// users who are already assigned are left as they are. With SendEmail, every
// listed user is emailed an invitation.
func (s *service) Assign(ctx context.Context, m authz.Membership, examID uuid.UUID, req AssignRequest) (*AssignResult, error) {
	var (
		exam   *examRef
		result = &AssignResult{}
	)
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if exam, err = s.repo.LockExam(ctx, m.OrgID, examID); err != nil {
			return err
		}
		switch exams.ExamStatus(exam.Status) {
		case exams.StatusDraft:
			if req.SendEmail {
				return errEmailNeedsPublish
			}
		case exams.StatusPublished:
		default:
			return errExamClosed
		}

		outsiders, err := s.repo.NonMembers(ctx, m.OrgID, req.UserIDs)
		if err != nil {
			return err
		}
		if len(outsiders) > 0 {
			ids := make([]string, len(outsiders))
			for i, id := range outsiders {
				ids[i] = id.String()
			}
			return apperr.Validation(map[string]string{
				"user_ids": "not members of this organization: " + strings.Join(ids, ", "),
			})
		}

		if result.Created, err = s.repo.Assign(ctx, examID, m.UserID, req.UserIDs); err != nil {
			return err
		}
		result.Assignments, err = s.repo.ListForUsers(ctx, examID, req.UserIDs)
		return err
	})
	if err != nil {
		return nil, s.mapErr(err, "assign candidates")
	}

	if req.SendEmail {
		result.Emailed = s.sendInvites(ctx, examID, exam, result.Assignments)
	}
	return result, nil
}

func (s *service) List(ctx context.Context, m authz.Membership, examID uuid.UUID, p pagination.Params) (pagination.Page[Assignment], error) {
	if _, err := s.repo.GetExam(ctx, m.OrgID, examID); err != nil {
		return pagination.Page[Assignment]{}, s.mapErr(err, "list assignments")
	}
	rows, err := s.repo.List(ctx, examID, p)
	if err != nil {
		return pagination.Page[Assignment]{}, s.mapErr(err, "list assignments")
	}
	return pagination.NewPage(rows, p.Limit, func(a Assignment) pagination.Cursor {
		return pagination.Cursor{Time: a.CreatedAt, ID: a.ID}
	}), nil
}

func (s *service) Unassign(ctx context.Context, m authz.Membership, examID, assignmentID uuid.UUID) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.repo.LockExam(ctx, m.OrgID, examID); err != nil {
			return err
		}
		return s.repo.Delete(ctx, examID, assignmentID)
	})
	return s.mapErr(err, "unassign candidate")
}

// sendInvites queues an invitation email per assignment and records which
// were queued, on both the rows and list. It returns how many were queued.
// Failures are logged, not returned: the assignments exist and the request
// can be repeated.
func (s *service) sendInvites(ctx context.Context, examID uuid.UUID, exam *examRef, list []Assignment) int {
	if s.opts.Mailer == nil {
		s.logger.Warn("assignments: send_email requested but no mailer is configured", "exam_id", examID)
		return 0
	}

	examURL := fmt.Sprintf("%s/exams/%s", s.opts.FrontendURL, examID)
	startTime := ""
	if exam.StartTime != nil && exams.ScheduleType(exam.ScheduleType) != exams.ScheduleOpenEnded {
		startTime = exam.StartTime.UTC().Format("Mon, 02 Jan 2006 15:04 MST")
	}
	duration := fmt.Sprintf("%d minutes", exam.TotalDurationMinutes)

	sent := make([]uuid.UUID, 0, len(list))
	sentIdx := make([]int, 0, len(list))
	for i, a := range list {
		name := a.FirstName
		if a.LastName != nil && *a.LastName != "" {
			name += " " + *a.LastName
		}
		if err := s.opts.Mailer.SendExamInviteEmail(a.Email, name, exam.Title, examURL, startTime, duration, ""); err != nil {
			s.logger.Warn("assignments: queue invitation email", "assignment_id", a.ID, "error", err)
			continue
		}
		sent = append(sent, a.ID)
		sentIdx = append(sentIdx, i)
	}
	if len(sent) > 0 {
		at := time.Now().UTC()
		if err := s.repo.MarkInviteSent(ctx, sent, at); err != nil {
			s.logger.Warn("assignments: record invitation emails", "exam_id", examID, "error", err)
		} else {
			for _, i := range sentIdx {
				list[i].InviteEmailSentAt = &at
			}
		}
	}
	return len(sent)
}

func (s *service) mapErr(err error, op string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrExamNotFound):
		return errExamNotFound
	case errors.Is(err, ErrAssignmentNotFound):
		return errAssignmentNotFound
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	s.logger.Error("assignments: "+op+" failed", "error", err)
	return apperr.Internal(err, op)
}
