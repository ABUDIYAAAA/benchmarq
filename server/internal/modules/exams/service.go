package exams

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/pagination"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/request"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/validator"
	"github.com/google/uuid"
)

// MaxSectionsPerExam bounds exam size to keep validation and payloads cheap.
const MaxSectionsPerExam = 50

var (
	errExamNotFound    = apperr.NotFound("Exam not found")
	errSectionNotFound = apperr.NotFound("Section not found")
	errExamCodeTaken   = apperr.Conflict("Another exam in this organization already uses this code")
)

// Transition is a lifecycle action on an exam.
type Transition string

const (
	TransitionPublish   Transition = "publish"
	TransitionUnpublish Transition = "unpublish"
	TransitionArchive   Transition = "archive"
	TransitionRestore   Transition = "restore"
)

// transitions is the exam lifecycle state machine: action -> allowed sources -> target.
var transitions = map[Transition]struct {
	from []ExamStatus
	to   ExamStatus
}{
	TransitionPublish:   {from: []ExamStatus{StatusDraft}, to: StatusPublished},
	TransitionUnpublish: {from: []ExamStatus{StatusPublished}, to: StatusDraft},
	TransitionArchive:   {from: []ExamStatus{StatusDraft, StatusPublished, StatusConcluded}, to: StatusArchived},
	TransitionRestore:   {from: []ExamStatus{StatusArchived}, to: StatusDraft},
}

type Service interface {
	CreateExam(ctx context.Context, m authz.Membership, req CreateExamRequest) (*ExamDetail, error)
	ListExams(ctx context.Context, m authz.Membership, f ExamFilter, p pagination.Params) (pagination.Page[Exam], error)
	GetExam(ctx context.Context, m authz.Membership, examID uuid.UUID) (*ExamDetail, error)
	UpdateExam(ctx context.Context, m authz.Membership, examID uuid.UUID, patch json.RawMessage) (*Exam, error)
	DeleteExam(ctx context.Context, m authz.Membership, examID uuid.UUID) error

	GetConfig(ctx context.Context, m authz.Membership, examID uuid.UUID) (*ConfigResponse, error)
	UpdateConfig(ctx context.Context, m authz.Membership, examID uuid.UUID, patch json.RawMessage) (*ConfigResponse, error)

	Readiness(ctx context.Context, m authz.Membership, examID uuid.UUID) (*Readiness, error)
	Transition(ctx context.Context, m authz.Membership, examID uuid.UUID, action Transition) (*Exam, error)

	ListSections(ctx context.Context, m authz.Membership, examID uuid.UUID) ([]SectionView, error)
	GetSection(ctx context.Context, m authz.Membership, examID, sectionID uuid.UUID) (*SectionView, error)
	CreateSection(ctx context.Context, m authz.Membership, examID uuid.UUID, req CreateSectionRequest) (*SectionView, error)
	UpdateSection(ctx context.Context, m authz.Membership, examID, sectionID uuid.UUID, patch json.RawMessage) (*SectionView, error)
	DeleteSection(ctx context.Context, m authz.Membership, examID, sectionID uuid.UUID) error
	ReorderSections(ctx context.Context, m authz.Membership, examID uuid.UUID, ordered []uuid.UUID) ([]SectionView, error)
}

type service struct {
	repo   Repository
	tx     database.Transactor
	logger *slog.Logger
	now    func() time.Time
}

func NewService(repo Repository, tx database.Transactor, logger *slog.Logger) Service {
	return &service{repo: repo, tx: tx, logger: logger, now: func() time.Time { return time.Now().UTC() }}
}

// ---------------------------------------------------------------------------
// Exams
// ---------------------------------------------------------------------------

func (s *service) CreateExam(ctx context.Context, m authz.Membership, req CreateExamRequest) (*ExamDetail, error) {
	exam := &Exam{
		OrgID:           m.OrgID,
		CreatedByUserID: m.UserID,
		Status:          StatusDraft,
	}
	req.ExamInput.applyTo(exam)
	normalizeExam(exam)

	var cfg *ExamConfig
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.repo.CreateExam(ctx, exam); err != nil {
			return err
		}
		var err error
		if cfg, err = s.repo.CreateDefaultConfig(ctx, exam.ID); err != nil {
			return err
		}
		if len(req.Config) == 0 {
			return nil
		}
		if _, err := s.applyConfigPatch(cfg, req.Config, "config."); err != nil {
			return err
		}
		return s.repo.SaveConfig(ctx, cfg)
	})
	if err != nil {
		return nil, s.mapErr(err, "create exam")
	}
	return &ExamDetail{Exam: exam, Config: cfg, Sections: []SectionView{}}, nil
}

func (s *service) ListExams(ctx context.Context, m authz.Membership, f ExamFilter, p pagination.Params) (pagination.Page[Exam], error) {
	rows, err := s.repo.ListExams(ctx, m.OrgID, f, p)
	if err != nil {
		return pagination.Page[Exam]{}, s.mapErr(err, "list exams")
	}
	return pagination.NewPage(rows, p.Limit, func(e Exam) pagination.Cursor {
		return pagination.Cursor{Time: e.CreatedAt, ID: e.ID}
	}), nil
}

func (s *service) GetExam(ctx context.Context, m authz.Membership, examID uuid.UUID) (*ExamDetail, error) {
	bp, err := s.loadBlueprint(ctx, m.OrgID, examID, false)
	if err != nil {
		return nil, s.mapErr(err, "get exam")
	}
	views := buildSectionViews(bp)
	if bp.Exam.Status == StatusDraft {
		// Drafts report live totals; published exams keep the frozen value.
		bp.Exam.TotalMarks = totalMarks(views)
	}
	return &ExamDetail{Exam: bp.Exam, Config: bp.Config, Sections: views}, nil
}

func (s *service) UpdateExam(ctx context.Context, m authz.Membership, examID uuid.UUID, patch json.RawMessage) (*Exam, error) {
	var exam *Exam
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if exam, err = s.lockDraft(ctx, m.OrgID, examID); err != nil {
			return err
		}
		in := examInputFrom(exam)
		if err := request.ApplyMergePatch(&in, patch); err != nil {
			return err
		}
		if fields, ok := validator.ValidateStruct(in); !ok {
			return apperr.Validation(fields)
		}
		in.applyTo(exam)
		normalizeExam(exam)
		return s.repo.UpdateExam(ctx, exam)
	})
	if err != nil {
		return nil, s.mapErr(err, "update exam")
	}
	return exam, nil
}

// DeleteExam soft-deletes an exam. Exams that candidates have attempted keep
// their history and must be archived instead.
func (s *service) DeleteExam(ctx context.Context, m authz.Membership, examID uuid.UUID) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		exam, err := s.repo.LockExam(ctx, m.OrgID, examID)
		if err != nil {
			return err
		}
		attempted, err := s.repo.HasAttempts(ctx, exam.ID)
		if err != nil {
			return err
		}
		if attempted {
			return apperr.Conflict("This exam has candidate attempts and cannot be deleted; archive it instead")
		}
		return s.repo.SoftDeleteExam(ctx, exam.ID)
	})
	return s.mapErr(err, "delete exam")
}

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

func (s *service) GetConfig(ctx context.Context, m authz.Membership, examID uuid.UUID) (*ConfigResponse, error) {
	if _, err := s.repo.GetExam(ctx, m.OrgID, examID); err != nil {
		return nil, s.mapErr(err, "get exam config")
	}
	cfg, err := s.repo.GetConfig(ctx, examID)
	if err != nil {
		return nil, s.mapErr(err, "get exam config")
	}
	_, warnings := splitIssues(ValidateConfig(cfg, false))
	return &ConfigResponse{Config: cfg, Warnings: nonNil(warnings)}, nil
}

// UpdateConfig merge-patches the configuration. Changes that would make an
// existing section invalid (e.g. shrinking the duration below a section's
// time limit) are rejected here rather than surfacing only at publish time.
func (s *service) UpdateConfig(ctx context.Context, m authz.Membership, examID uuid.UUID, patch json.RawMessage) (*ConfigResponse, error) {
	var (
		cfg      *ExamConfig
		warnings []Issue
	)
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.lockDraft(ctx, m.OrgID, examID); err != nil {
			return err
		}
		var err error
		if cfg, err = s.repo.GetConfig(ctx, examID); err != nil {
			return err
		}
		if warnings, err = s.applyConfigPatch(cfg, patch, ""); err != nil {
			return err
		}

		sections, err := s.repo.ListSections(ctx, examID)
		if err != nil {
			return err
		}
		var is []Issue
		for i := range sections {
			is = append(is, ValidateSectionAgainstConfig(cfg, &sections[i])...)
		}
		if hasErrors(is) {
			return apperr.ValidationWithMessage("Configuration conflicts with existing sections", issueFields(is))
		}
		return s.repo.SaveConfig(ctx, cfg)
	})
	if err != nil {
		return nil, s.mapErr(err, "update exam config")
	}
	return &ConfigResponse{Config: cfg, Warnings: nonNil(warnings)}, nil
}

// applyConfigPatch merges patch into cfg and validates the result. Paths in
// the returned errors are prefixed with pathPrefix (and "config." stripped
// from rule paths when the prefix is empty, as the body is the config itself).
func (s *service) applyConfigPatch(cfg *ExamConfig, patch json.RawMessage, pathPrefix string) ([]Issue, error) {
	examID, createdAt, updatedAt := cfg.ExamID, cfg.CreatedAt, cfg.UpdatedAt
	if err := request.ApplyMergePatch(cfg, patch); err != nil {
		return nil, err
	}
	cfg.ExamID, cfg.CreatedAt, cfg.UpdatedAt = examID, createdAt, updatedAt
	normalizeConfig(cfg)

	if fields, ok := validator.ValidateStruct(cfg); !ok {
		prefixed := make(map[string]string, len(fields))
		for k, v := range fields {
			prefixed[pathPrefix+k] = v
		}
		return nil, apperr.Validation(prefixed)
	}

	is := ValidateConfig(cfg, false)
	if pathPrefix == "" {
		for i := range is {
			is[i].Path = strings.TrimPrefix(is[i].Path, "config.")
		}
	}
	errs, warnings := splitIssues(is)
	if len(errs) > 0 {
		return nil, apperr.ValidationWithMessage("Invalid exam configuration", issueFields(errs))
	}
	return warnings, nil
}

// ---------------------------------------------------------------------------
// Readiness and lifecycle
// ---------------------------------------------------------------------------

func (s *service) Readiness(ctx context.Context, m authz.Membership, examID uuid.UUID) (*Readiness, error) {
	bp, err := s.loadBlueprint(ctx, m.OrgID, examID, false)
	if err != nil {
		return nil, s.mapErr(err, "assess exam readiness")
	}
	r := AssessReadiness(*bp)
	return &r, nil
}

func (s *service) Transition(ctx context.Context, m authz.Membership, examID uuid.UUID, action Transition) (*Exam, error) {
	rule, ok := transitions[action]
	if !ok {
		return nil, apperr.Invalid("Unknown exam action")
	}

	var exam *Exam
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		bp, err := s.loadBlueprint(ctx, m.OrgID, examID, true)
		if err != nil {
			return err
		}
		exam = bp.Exam
		if !slices.Contains(rule.from, exam.Status) {
			return apperr.Conflict(fmt.Sprintf("Cannot %s an exam that is %s", action, exam.Status))
		}

		switch action {
		case TransitionPublish:
			readiness := AssessReadiness(*bp)
			if !readiness.Ready {
				return apperr.ValidationWithMessage("Exam is not ready to publish", issueFields(readiness.Errors))
			}
			now := s.now()
			exam.TotalMarks = readiness.TotalMarks
			exam.PublishedAt = &now

		case TransitionUnpublish, TransitionRestore:
			// Back to draft means editable again, which would corrupt the
			// grading of anyone who already sat the exam.
			attempted, err := s.repo.HasAttempts(ctx, exam.ID)
			if err != nil {
				return err
			}
			if attempted {
				return apperr.Conflict("Candidates have already attempted this exam, so it can no longer be edited")
			}
			exam.PublishedAt = nil
		}

		exam.Status = rule.to
		return s.repo.UpdateExam(ctx, exam)
	})
	if err != nil {
		return nil, s.mapErr(err, string(action)+" exam")
	}
	return exam, nil
}

// ---------------------------------------------------------------------------
// Sections
// ---------------------------------------------------------------------------

func (s *service) ListSections(ctx context.Context, m authz.Membership, examID uuid.UUID) ([]SectionView, error) {
	bp, err := s.loadBlueprint(ctx, m.OrgID, examID, false)
	if err != nil {
		return nil, s.mapErr(err, "list sections")
	}
	return buildSectionViews(bp), nil
}

func (s *service) GetSection(ctx context.Context, m authz.Membership, examID, sectionID uuid.UUID) (*SectionView, error) {
	views, err := s.ListSections(ctx, m, examID)
	if err != nil {
		return nil, err
	}
	return findView(views, sectionID)
}

func (s *service) CreateSection(ctx context.Context, m authz.Membership, examID uuid.UUID, req CreateSectionRequest) (*SectionView, error) {
	var views []SectionView
	section := &Section{ID: uuid.New(), ExamID: examID}
	req.SectionInput.applyTo(section)

	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		bp, err := s.loadDraftBlueprint(ctx, m.OrgID, examID)
		if err != nil {
			return err
		}
		n := len(bp.Sections)
		if n >= MaxSectionsPerExam {
			return apperr.Conflict(fmt.Sprintf("An exam can have at most %d sections", MaxSectionsPerExam))
		}

		section.Position = n
		if req.Position != nil && *req.Position < n {
			section.Position = *req.Position
		}
		after := slices.Insert(slices.Clone(bp.Sections), section.Position, *section)
		if err := validateSectionWrite(bp.Config, after, section); err != nil {
			return err
		}

		if section.Position < n {
			if err := s.repo.ShiftSections(ctx, examID, section.Position, 1); err != nil {
				return err
			}
		}
		if err := s.repo.CreateSection(ctx, section); err != nil {
			return err
		}
		views, err = s.sectionViews(ctx, m.OrgID, examID)
		return err
	})
	if err != nil {
		return nil, s.mapErr(err, "create section")
	}
	return findView(views, section.ID)
}

func (s *service) UpdateSection(ctx context.Context, m authz.Membership, examID, sectionID uuid.UUID, patch json.RawMessage) (*SectionView, error) {
	var views []SectionView
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		bp, err := s.loadDraftBlueprint(ctx, m.OrgID, examID)
		if err != nil {
			return err
		}
		idx := slices.IndexFunc(bp.Sections, func(sec Section) bool { return sec.ID == sectionID })
		if idx < 0 {
			return ErrSectionNotFound
		}
		section := &bp.Sections[idx]

		in := sectionInputFrom(section)
		if err := request.ApplyMergePatch(&in, patch); err != nil {
			return err
		}
		if fields, ok := validator.ValidateStruct(in); !ok {
			return apperr.Validation(fields)
		}
		if in.SectionType != section.SectionType {
			// Questions are bound to their section's family by a composite FK.
			return apperr.Validation(map[string]string{"section_type": "section_type cannot be changed after the section is created"})
		}
		in.applyTo(section)

		if err := validateSectionWrite(bp.Config, bp.Sections, section); err != nil {
			return err
		}
		if err := s.repo.UpdateSection(ctx, section); err != nil {
			return err
		}
		views, err = s.sectionViews(ctx, m.OrgID, examID)
		return err
	})
	if err != nil {
		return nil, s.mapErr(err, "update section")
	}
	return findView(views, sectionID)
}

// DeleteSection removes a section and its questions, then closes the gap in
// positions. Prerequisite links pointing at the section are dropped by
// cascade.
func (s *service) DeleteSection(ctx context.Context, m authz.Membership, examID, sectionID uuid.UUID) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		bp, err := s.loadDraftBlueprint(ctx, m.OrgID, examID)
		if err != nil {
			return err
		}
		idx := slices.IndexFunc(bp.Sections, func(sec Section) bool { return sec.ID == sectionID })
		if idx < 0 {
			return ErrSectionNotFound
		}
		if err := s.repo.DeleteSection(ctx, examID, sectionID); err != nil {
			return err
		}
		return s.repo.ShiftSections(ctx, examID, bp.Sections[idx].Position+1, -1)
	})
	return s.mapErr(err, "delete section")
}

// ReorderSections applies a complete new ordering. The request must list
// every section exactly once, which rules out lost or duplicated positions.
func (s *service) ReorderSections(ctx context.Context, m authz.Membership, examID uuid.UUID, ordered []uuid.UUID) ([]SectionView, error) {
	var views []SectionView
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		bp, err := s.loadDraftBlueprint(ctx, m.OrgID, examID)
		if err != nil {
			return err
		}
		if !isPermutation(bp.Sections, ordered) {
			return apperr.Validation(map[string]string{"section_ids": "must list every section of the exam exactly once"})
		}
		if err := s.repo.SetSectionOrder(ctx, examID, ordered); err != nil {
			return err
		}
		views, err = s.sectionViews(ctx, m.OrgID, examID)
		return err
	})
	if err != nil {
		return nil, s.mapErr(err, "reorder sections")
	}
	return views, nil
}

// validateSectionWrite runs the write-time checks for section after a change:
// configuration limits and the prerequisite graph. sections must already
// contain the changed section.
func validateSectionWrite(cfg *ExamConfig, sections []Section, section *Section) error {
	is := ValidateSectionAgainstConfig(cfg, section)
	is = append(is, ValidatePrerequisites(sections)...)
	if !hasErrors(is) {
		return nil
	}
	// Report paths relative to the request body.
	prefix := "sections." + section.ID.String() + "."
	fields := issueFields(is)
	rel := make(map[string]string, len(fields))
	for k, v := range fields {
		rel[strings.TrimPrefix(k, prefix)] = v
	}
	return apperr.Validation(rel)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *service) loadBlueprint(ctx context.Context, orgID, examID uuid.UUID, lock bool) (*Blueprint, error) {
	var (
		exam *Exam
		err  error
	)
	if lock {
		exam, err = s.repo.LockExam(ctx, orgID, examID)
	} else {
		exam, err = s.repo.GetExam(ctx, orgID, examID)
	}
	if err != nil {
		return nil, err
	}
	cfg, err := s.repo.GetConfig(ctx, examID)
	if err != nil {
		return nil, err
	}
	sections, err := s.repo.ListSections(ctx, examID)
	if err != nil {
		return nil, err
	}
	stats, err := s.repo.SectionStats(ctx, examID)
	if err != nil {
		return nil, err
	}
	return &Blueprint{Exam: exam, Config: cfg, Sections: sections, Stats: stats}, nil
}

// loadDraftBlueprint locks the exam and ensures it is still editable.
func (s *service) loadDraftBlueprint(ctx context.Context, orgID, examID uuid.UUID) (*Blueprint, error) {
	bp, err := s.loadBlueprint(ctx, orgID, examID, true)
	if err != nil {
		return nil, err
	}
	if err := EnsureEditable(bp.Exam.Status); err != nil {
		return nil, err
	}
	return bp, nil
}

func (s *service) lockDraft(ctx context.Context, orgID, examID uuid.UUID) (*Exam, error) {
	exam, err := s.repo.LockExam(ctx, orgID, examID)
	if err != nil {
		return nil, err
	}
	if err := EnsureEditable(exam.Status); err != nil {
		return nil, err
	}
	return exam, nil
}

func (s *service) sectionViews(ctx context.Context, orgID, examID uuid.UUID) ([]SectionView, error) {
	bp, err := s.loadBlueprint(ctx, orgID, examID, false)
	if err != nil {
		return nil, err
	}
	return buildSectionViews(bp), nil
}

// EnsureEditable enforces the core authoring invariant: only drafts change.
// Published exams are immutable so every candidate sees the same exam.
func EnsureEditable(status ExamStatus) error {
	switch status {
	case StatusDraft:
		return nil
	case StatusPublished:
		return apperr.Conflict("Exam is published; unpublish it before editing")
	case StatusArchived:
		return apperr.Conflict("Exam is archived; restore it before editing")
	default:
		return apperr.Conflict(fmt.Sprintf("Exam is %s and can no longer be edited", status))
	}
}

func buildSectionViews(bp *Blueprint) []SectionView {
	rules := ResolveRules(bp.Config, bp.Sections)
	views := make([]SectionView, len(bp.Sections))
	for i, sec := range bp.Sections {
		stats := bp.Stats[sec.ID]
		views[i] = SectionView{
			Section:       sec,
			QuestionCount: stats.QuestionCount,
			MaxMarks:      SectionMaxMarks(rules[sec.ID], stats),
			Effective:     rules[sec.ID],
		}
	}
	return views
}

func totalMarks(views []SectionView) float64 {
	var total float64
	for _, v := range views {
		total += v.MaxMarks
	}
	return round2(total)
}

func findView(views []SectionView, id uuid.UUID) (*SectionView, error) {
	for i := range views {
		if views[i].ID == id {
			return &views[i], nil
		}
	}
	return nil, errSectionNotFound
}

func isPermutation(sections []Section, ids []uuid.UUID) bool {
	if len(sections) != len(ids) {
		return false
	}
	want := make(map[uuid.UUID]bool, len(sections))
	for _, s := range sections {
		want[s.ID] = true
	}
	for _, id := range ids {
		if !want[id] {
			return false
		}
		delete(want, id)
	}
	return len(want) == 0
}

func normalizeExam(e *Exam) {
	e.Title = strings.TrimSpace(e.Title)
	e.Code = trimmedOrNil(e.Code)
	e.Description = trimmedOrNil(e.Description)
	e.Instructions = trimmedOrNil(e.Instructions)
}

func normalizeConfig(cfg *ExamConfig) {
	// Warnings fire from the longest threshold to the shortest.
	slices.SortFunc(cfg.Experience.TimerWarningThresholdsMinutes, func(a, b int32) int { return int(b - a) })
	cfg.Proctoring.SEBConfigKey = trimmedOrNil(cfg.Proctoring.SEBConfigKey)
	if cfg.Proctoring.IPWhitelistCIDR != nil && len(cfg.Proctoring.IPWhitelistCIDR) == 0 {
		cfg.Proctoring.IPWhitelistCIDR = nil
	}
	if cfg.Coding.AllowedLanguageIDs != nil && len(cfg.Coding.AllowedLanguageIDs) == 0 {
		cfg.Coding.AllowedLanguageIDs = nil
	}
}

func trimmedOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

func nonNil(list []Issue) []Issue {
	if list == nil {
		return []Issue{}
	}
	return list
}

func (s *service) mapErr(err error, op string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrExamNotFound):
		return errExamNotFound
	case errors.Is(err, ErrSectionNotFound):
		return errSectionNotFound
	case errors.Is(err, ErrExamCodeTaken):
		return errExamCodeTaken
	case database.IsForeignKeyViolation(err, "exam_section_prerequisites_prerequisite_section_id_fkey"):
		return apperr.Validation(map[string]string{"prerequisites": "prerequisite section does not exist"})
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	s.logger.Error("exams: "+op+" failed", "error", err)
	return apperr.Internal(err, op)
}
