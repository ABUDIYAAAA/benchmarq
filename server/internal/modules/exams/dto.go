package exams

import (
	"encoding/json"

	"github.com/google/uuid"
)

// ExamInput is the writable view of an exam's metadata: the body of create
// requests and the merge target of PATCH requests.
type ExamInput struct {
	Title        string   `json:"title" validate:"required,min=1,max=255"`
	Code         *string  `json:"code" validate:"omitempty,min=1,max=50,printascii"`
	Description  *string  `json:"description" validate:"omitempty,max=10000"`
	Instructions *string  `json:"instructions" validate:"omitempty,max=50000"`
	PassingMarks *float64 `json:"passing_marks" validate:"omitempty,gte=0"`
}

func examInputFrom(e *Exam) ExamInput {
	return ExamInput{
		Title:        e.Title,
		Code:         e.Code,
		Description:  e.Description,
		Instructions: e.Instructions,
		PassingMarks: e.PassingMarks,
	}
}

func (in ExamInput) applyTo(e *Exam) {
	e.Title = in.Title
	e.Code = in.Code
	e.Description = in.Description
	e.Instructions = in.Instructions
	e.PassingMarks = in.PassingMarks
}

type CreateExamRequest struct {
	ExamInput
	// Config is an optional partial configuration merged over the defaults.
	Config json.RawMessage `json:"config,omitempty"`
}

type ExamFilter struct {
	Status *ExamStatus
	Search string
}

// SectionInput is the writable view of a section.
type SectionInput struct {
	Name               string              `json:"name" validate:"required,min=1,max=255"`
	Description        *string             `json:"description" validate:"omitempty,max=10000"`
	Instructions       *string             `json:"instructions" validate:"omitempty,max=50000"`
	SectionType        SectionType         `json:"section_type" validate:"required,oneof=objective coding subjective"`
	DurationMinutes    *int                `json:"duration_minutes" validate:"omitempty,gte=1,lte=1440"`
	MarksWeightage     *float64            `json:"marks_weightage" validate:"omitempty,gte=0,lte=1000"`
	CutoffMarks        *float64            `json:"cutoff_marks" validate:"omitempty,gte=0"`
	ShuffleQuestions   *bool               `json:"shuffle_questions"`
	ShuffleOptions     *bool               `json:"shuffle_options"`
	QuestionPickCount  *int                `json:"question_pick_count" validate:"omitempty,gte=1,lte=1000"`
	UnlockAfterMinutes *int                `json:"unlock_after_minutes" validate:"omitempty,gte=0,lte=1440"`
	LockOnComplete     bool                `json:"lock_on_complete"`
	Prerequisites      []PrerequisiteInput `json:"prerequisites" validate:"omitempty,max=50,dive"`
}

type PrerequisiteInput struct {
	SectionID       uuid.UUID `json:"section_id" validate:"required"`
	MinScorePercent *float64  `json:"min_score_percent" validate:"omitempty,gt=0,lte=100"`
}

func sectionInputFrom(s *Section) SectionInput {
	prereqs := make([]PrerequisiteInput, len(s.Prerequisites))
	for i, p := range s.Prerequisites {
		prereqs[i] = PrerequisiteInput(p)
	}
	return SectionInput{
		Name:               s.Name,
		Description:        s.Description,
		Instructions:       s.Instructions,
		SectionType:        s.SectionType,
		DurationMinutes:    s.DurationMinutes,
		MarksWeightage:     s.MarksWeightage,
		CutoffMarks:        s.CutoffMarks,
		ShuffleQuestions:   s.ShuffleQuestions,
		ShuffleOptions:     s.ShuffleOptions,
		QuestionPickCount:  s.QuestionPickCount,
		UnlockAfterMinutes: s.UnlockAfterMinutes,
		LockOnComplete:     s.LockOnComplete,
		Prerequisites:      prereqs,
	}
}

func (in SectionInput) applyTo(s *Section) {
	s.Name = in.Name
	s.Description = in.Description
	s.Instructions = in.Instructions
	s.SectionType = in.SectionType
	s.DurationMinutes = in.DurationMinutes
	s.MarksWeightage = in.MarksWeightage
	s.CutoffMarks = in.CutoffMarks
	s.ShuffleQuestions = in.ShuffleQuestions
	s.ShuffleOptions = in.ShuffleOptions
	s.QuestionPickCount = in.QuestionPickCount
	s.UnlockAfterMinutes = in.UnlockAfterMinutes
	s.LockOnComplete = in.LockOnComplete
	s.Prerequisites = make([]Prerequisite, len(in.Prerequisites))
	for i, p := range in.Prerequisites {
		s.Prerequisites[i] = Prerequisite(p)
	}
}

type CreateSectionRequest struct {
	SectionInput
	// Position inserts the section at this zero-based index, shifting later
	// sections down. Omit to append.
	Position *int `json:"position" validate:"omitempty,gte=0"`
}

type ReorderRequest struct {
	SectionIDs []uuid.UUID `json:"section_ids" validate:"required,min=1,unique"`
}

// SectionView is a section with derived information for the authoring UI.
type SectionView struct {
	Section
	QuestionCount int            `json:"question_count"`
	MaxMarks      float64        `json:"max_marks"`
	Effective     EffectiveRules `json:"effective"`
}

type ExamDetail struct {
	Exam     *Exam         `json:"exam"`
	Config   *ExamConfig   `json:"config"`
	Sections []SectionView `json:"sections"`
}

type ConfigResponse struct {
	Config   *ExamConfig `json:"config"`
	Warnings []Issue     `json:"warnings"`
}
