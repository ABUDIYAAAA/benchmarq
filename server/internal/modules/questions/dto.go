package questions

import (
	"strings"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/google/uuid"
)

const (
	defaultCodingTimeLimitMs   = 2000
	defaultCodingMemoryLimitMb = 128
)

// QuestionInput is the full definition of a question. It is used for both
// create (POST) and replace (PUT): a question is small and self-contained, so
// whole-document replacement is simpler and safer than field-level patches.
type QuestionInput struct {
	Type           QuestionType `json:"type" validate:"required,oneof=multiple_choice true_false multi_select blank_fill coding short_answer essay"`
	Prompt         string       `json:"prompt" validate:"required,max=20000"`
	Explanation    *string      `json:"explanation" validate:"omitempty,max=20000"`
	Points         float64      `json:"points" validate:"gt=0,lte=1000"`
	NegativePoints float64      `json:"negative_points" validate:"gte=0,lte=1000"`

	Options     []OptionInput     `json:"options" validate:"omitempty,max=26,dive"`
	TrueFalse   *TrueFalseInput   `json:"true_false"`
	MultiSelect *MultiSelectInput `json:"multi_select"`
	BlankFill   *BlankFillInput   `json:"blank_fill"`
	ShortAnswer *ShortAnswerInput `json:"short_answer"`
	Essay       *EssayInput       `json:"essay"`
	Coding      *CodingInput      `json:"coding"`
}

type OptionInput struct {
	Content   string   `json:"content" validate:"required,max=5000"`
	IsCorrect bool     `json:"is_correct"`
	IsPinned  bool     `json:"is_pinned"`
	Weight    *float64 `json:"weight" validate:"omitempty,gte=0,lte=100"`
}

type TrueFalseInput struct {
	CorrectAnswer *bool `json:"correct_answer" validate:"required"`
}

type MultiSelectInput struct {
	MinChoicesRequired *int `json:"min_choices_required" validate:"omitempty,gte=1,lte=26"`
	MaxChoicesAllowed  *int `json:"max_choices_allowed" validate:"omitempty,gte=1,lte=26"`
}

type BlankFillInput struct {
	CorrectAnswer    string   `json:"correct_answer" validate:"required,max=1000"`
	CaseSensitive    bool     `json:"case_sensitive"`
	AlternateAnswers []string `json:"alternate_answers" validate:"omitempty,max=50,dive,required,max=1000"`
}

type ShortAnswerInput struct {
	SampleAnswer   *string  `json:"sample_answer" validate:"omitempty,max=20000"`
	MaxLength      *int     `json:"max_length" validate:"omitempty,gte=1,lte=100000"`
	MaxWords       *int     `json:"max_words" validate:"omitempty,gte=1,lte=20000"`
	RubricKeywords []string `json:"rubric_keywords" validate:"omitempty,max=100,dive,required,max=200"`
}

type EssayInput struct {
	RubricGuidelines     *string `json:"rubric_guidelines" validate:"omitempty,max=20000"`
	MinWords             *int    `json:"min_words" validate:"omitempty,gte=0,lte=20000"`
	MaxWords             *int    `json:"max_words" validate:"omitempty,gte=1,lte=20000"`
	MaxLength            *int    `json:"max_length" validate:"omitempty,gte=1,lte=500000"`
	AllowRichText        *bool   `json:"allow_rich_text"`
	AllowFileAttachments bool    `json:"allow_file_attachments"`
}

type CodingInput struct {
	DefaultLanguageID *uuid.UUID      `json:"default_language_id"`
	StarterCode       *string         `json:"starter_code" validate:"omitempty,max=100000"`
	SolutionCode      *string         `json:"solution_code" validate:"omitempty,max=100000"`
	DriverCode        *string         `json:"driver_code" validate:"omitempty,max=100000"`
	TimeLimitMs       *int            `json:"time_limit_ms" validate:"omitempty,gte=100,lte=60000"`
	MemoryLimitMb     *int            `json:"memory_limit_mb" validate:"omitempty,gte=16,lte=4096"`
	TestCases         []TestCaseInput `json:"test_cases" validate:"required,min=1,max=200,dive"`
}

type TestCaseInput struct {
	Input               string   `json:"input" validate:"max=1000000"`
	ExpectedOutput      string   `json:"expected_output" validate:"max=1000000"`
	Points              *float64 `json:"points" validate:"omitempty,gt=0,lte=1000"`
	IsHidden            *bool    `json:"is_hidden"`
	TimeLimitOverrideMs *int     `json:"time_limit_override_ms" validate:"omitempty,gte=100,lte=60000"`
}

type CreateQuestionRequest struct {
	QuestionInput
	// Position inserts the question at this zero-based index. Omit to append.
	Position *int `json:"position" validate:"omitempty,gte=0"`
}

type ReorderRequest struct {
	QuestionIDs []uuid.UUID `json:"question_ids" validate:"required,min=1,unique"`
}

// build converts validated input into a Question, generating IDs for options
// and test cases and applying defaults. Options and test cases are ordered as
// submitted.
func build(in *QuestionInput, sectionType exams.SectionType) *Question {
	q := &Question{
		SectionType:    sectionType,
		Type:           in.Type,
		Prompt:         strings.TrimSpace(in.Prompt),
		Explanation:    trimmedOrNil(in.Explanation),
		Points:         in.Points,
		NegativePoints: in.NegativePoints,
	}

	if in.Type.usesOptions() {
		q.Options = make([]Option, len(in.Options))
		for i, o := range in.Options {
			weight := 1.0
			if o.Weight != nil {
				weight = *o.Weight
			}
			q.Options[i] = Option{
				ID:        uuid.New(),
				Content:   strings.TrimSpace(o.Content),
				Position:  i,
				IsCorrect: o.IsCorrect,
				IsPinned:  o.IsPinned,
				Weight:    weight,
			}
		}
	}

	switch in.Type {
	case TypeTrueFalse:
		q.TrueFalse = &TrueFalseSpec{CorrectAnswer: *in.TrueFalse.CorrectAnswer}

	case TypeMultiSelect:
		spec := &MultiSelectSpec{MinChoicesRequired: 1}
		if in.MultiSelect != nil {
			if in.MultiSelect.MinChoicesRequired != nil {
				spec.MinChoicesRequired = *in.MultiSelect.MinChoicesRequired
			}
			spec.MaxChoicesAllowed = in.MultiSelect.MaxChoicesAllowed
		}
		q.MultiSelect = spec

	case TypeBlankFill:
		q.BlankFill = &BlankFillSpec{
			CorrectAnswer:    strings.TrimSpace(in.BlankFill.CorrectAnswer),
			CaseSensitive:    in.BlankFill.CaseSensitive,
			AlternateAnswers: trimAll(in.BlankFill.AlternateAnswers),
		}

	case TypeShortAnswer:
		spec := &ShortAnswerSpec{RubricKeywords: []string{}}
		if sa := in.ShortAnswer; sa != nil {
			spec.SampleAnswer = trimmedOrNil(sa.SampleAnswer)
			spec.MaxLength = sa.MaxLength
			spec.MaxWords = sa.MaxWords
			spec.RubricKeywords = trimAll(sa.RubricKeywords)
		}
		q.ShortAnswer = spec

	case TypeEssay:
		spec := &EssaySpec{AllowRichText: true}
		if e := in.Essay; e != nil {
			spec.RubricGuidelines = trimmedOrNil(e.RubricGuidelines)
			spec.MinWords = e.MinWords
			spec.MaxWords = e.MaxWords
			spec.MaxLength = e.MaxLength
			spec.AllowFileAttachments = e.AllowFileAttachments
			if e.AllowRichText != nil {
				spec.AllowRichText = *e.AllowRichText
			}
		}
		q.Essay = spec

	case TypeCoding:
		c := in.Coding
		spec := &CodingSpec{
			DefaultLanguageID: c.DefaultLanguageID,
			StarterCode:       c.StarterCode,
			SolutionCode:      c.SolutionCode,
			DriverCode:        c.DriverCode,
			TimeLimitMs:       valueOr(c.TimeLimitMs, defaultCodingTimeLimitMs),
			MemoryLimitMb:     valueOr(c.MemoryLimitMb, defaultCodingMemoryLimitMb),
			TestCases:         make([]TestCase, len(c.TestCases)),
		}
		for i, tc := range c.TestCases {
			hidden := true
			if tc.IsHidden != nil {
				hidden = *tc.IsHidden
			}
			points := 1.0
			if tc.Points != nil {
				points = *tc.Points
			}
			spec.TestCases[i] = TestCase{
				ID:                  uuid.New(),
				Input:               tc.Input,
				ExpectedOutput:      tc.ExpectedOutput,
				Points:              points,
				Position:            i,
				IsHidden:            hidden,
				TimeLimitOverrideMs: tc.TimeLimitOverrideMs,
			}
		}
		q.Coding = spec
	}
	return q
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

func trimAll(list []string) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

func valueOr(v *int, fallback int) int {
	if v == nil {
		return fallback
	}
	return *v
}
