package questions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/google/uuid"
)

// MaxQuestionsPerSection bounds section size.
const MaxQuestionsPerSection = 500

var (
	errSectionNotFound  = apperr.NotFound("Section not found")
	errQuestionNotFound = apperr.NotFound("Question not found")
	errUnknownLanguage  = apperr.Validation(map[string]string{"coding.default_language_id": "coding language does not exist"})
)

// Scope identifies a section within an organization's exam.
type Scope struct {
	OrgID     uuid.UUID
	ExamID    uuid.UUID
	SectionID uuid.UUID
}

type Service interface {
	List(ctx context.Context, scope Scope) ([]Question, error)
	Get(ctx context.Context, scope Scope, questionID uuid.UUID) (*Question, error)
	Create(ctx context.Context, scope Scope, req CreateQuestionRequest) (*Question, error)
	Replace(ctx context.Context, scope Scope, questionID uuid.UUID, in QuestionInput) (*Question, error)
	Delete(ctx context.Context, scope Scope, questionID uuid.UUID) error
	Reorder(ctx context.Context, scope Scope, ordered []uuid.UUID) ([]Question, error)
}

type service struct {
	repo   Repository
	tx     database.Transactor
	logger *slog.Logger
}

func NewService(repo Repository, tx database.Transactor, logger *slog.Logger) Service {
	return &service{repo: repo, tx: tx, logger: logger}
}

// ScopeFrom builds a Scope from the caller's membership.
func ScopeFrom(m authz.Membership, examID, sectionID uuid.UUID) Scope {
	return Scope{OrgID: m.OrgID, ExamID: examID, SectionID: sectionID}
}

func (s *service) List(ctx context.Context, scope Scope) ([]Question, error) {
	if _, err := s.repo.GetSection(ctx, scope.OrgID, scope.ExamID, scope.SectionID); err != nil {
		return nil, s.mapErr(err, "list questions")
	}
	qs, err := s.repo.List(ctx, scope.SectionID)
	if err != nil {
		return nil, s.mapErr(err, "list questions")
	}
	if qs == nil {
		qs = []Question{}
	}
	return qs, nil
}

func (s *service) Get(ctx context.Context, scope Scope, questionID uuid.UUID) (*Question, error) {
	if _, err := s.repo.GetSection(ctx, scope.OrgID, scope.ExamID, scope.SectionID); err != nil {
		return nil, s.mapErr(err, "get question")
	}
	q, err := s.repo.Get(ctx, scope.SectionID, questionID)
	if err != nil {
		return nil, s.mapErr(err, "get question")
	}
	return q, nil
}

func (s *service) Create(ctx context.Context, scope Scope, req CreateQuestionRequest) (*Question, error) {
	var created *Question
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		ref, err := s.lockEditableSection(ctx, scope)
		if err != nil {
			return err
		}
		if fields := Validate(ref.SectionType, &req.QuestionInput); len(fields) > 0 {
			return apperr.Validation(fields)
		}

		ids, err := s.repo.ListIDs(ctx, scope.SectionID)
		if err != nil {
			return err
		}
		n := len(ids)
		if n >= MaxQuestionsPerSection {
			return apperr.Conflict(fmt.Sprintf("A section can have at most %d questions", MaxQuestionsPerSection))
		}

		q := build(&req.QuestionInput, ref.SectionType)
		q.ID = uuid.New()
		q.SectionID = scope.SectionID
		q.Position = n
		if req.Position != nil && *req.Position < n {
			q.Position = *req.Position
			if err := s.repo.ShiftPositions(ctx, scope.SectionID, q.Position, 1); err != nil {
				return err
			}
		}
		if err := s.repo.Insert(ctx, q); err != nil {
			return err
		}
		created, err = s.repo.Get(ctx, scope.SectionID, q.ID)
		return err
	})
	if err != nil {
		return nil, s.mapErr(err, "create question")
	}
	return created, nil
}

func (s *service) Replace(ctx context.Context, scope Scope, questionID uuid.UUID, in QuestionInput) (*Question, error) {
	var updated *Question
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		ref, err := s.lockEditableSection(ctx, scope)
		if err != nil {
			return err
		}
		if fields := Validate(ref.SectionType, &in); len(fields) > 0 {
			return apperr.Validation(fields)
		}

		q := build(&in, ref.SectionType)
		q.ID = questionID
		q.SectionID = scope.SectionID
		if err := s.repo.Replace(ctx, q); err != nil {
			return err
		}
		updated, err = s.repo.Get(ctx, scope.SectionID, questionID)
		return err
	})
	if err != nil {
		return nil, s.mapErr(err, "replace question")
	}
	return updated, nil
}

func (s *service) Delete(ctx context.Context, scope Scope, questionID uuid.UUID) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.lockEditableSection(ctx, scope); err != nil {
			return err
		}
		position, err := s.repo.Delete(ctx, scope.SectionID, questionID)
		if err != nil {
			return err
		}
		return s.repo.ShiftPositions(ctx, scope.SectionID, position+1, -1)
	})
	return s.mapErr(err, "delete question")
}

// Reorder applies a complete new ordering; every question must be listed once.
func (s *service) Reorder(ctx context.Context, scope Scope, ordered []uuid.UUID) ([]Question, error) {
	var qs []Question
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.lockEditableSection(ctx, scope); err != nil {
			return err
		}
		ids, err := s.repo.ListIDs(ctx, scope.SectionID)
		if err != nil {
			return err
		}
		if !samePermutation(ids, ordered) {
			return apperr.Validation(map[string]string{"question_ids": "must list every question of the section exactly once"})
		}
		if err := s.repo.SetOrder(ctx, scope.SectionID, ordered); err != nil {
			return err
		}
		qs, err = s.repo.List(ctx, scope.SectionID)
		return err
	})
	if err != nil {
		return nil, s.mapErr(err, "reorder questions")
	}
	return qs, nil
}

func (s *service) lockEditableSection(ctx context.Context, scope Scope) (*SectionRef, error) {
	ref, err := s.repo.LockSection(ctx, scope.OrgID, scope.ExamID, scope.SectionID)
	if err != nil {
		return nil, err
	}
	if err := exams.EnsureEditable(ref.ExamStatus); err != nil {
		return nil, err
	}
	return ref, nil
}

func samePermutation(current, proposed []uuid.UUID) bool {
	if len(current) != len(proposed) {
		return false
	}
	a, b := slices.Clone(current), slices.Clone(proposed)
	cmp := func(x, y uuid.UUID) int { return slices.Compare(x[:], y[:]) }
	slices.SortFunc(a, cmp)
	slices.SortFunc(b, cmp)
	return slices.Equal(a, b)
}

func (s *service) mapErr(err error, op string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrSectionNotFound):
		return errSectionNotFound
	case errors.Is(err, ErrQuestionNotFound):
		return errQuestionNotFound
	case errors.Is(err, ErrUnknownLanguage):
		return errUnknownLanguage
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	s.logger.Error("questions: "+op+" failed", "error", err)
	return apperr.Internal(err, op)
}
