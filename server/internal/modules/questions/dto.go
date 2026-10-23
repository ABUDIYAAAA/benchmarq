package questions

import (
	"strings"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/google/uuid"
)

// QuestionInput is the full definition of a question. It is used for both
// create (POST) and replace (PUT): a question is small and self-contained, so
// whole-document replacement is simpler and safer than field-level patches.
type QuestionInput struct {
	Type        QuestionType `json:"type" validate:"required,oneof=multiple_choice true_false multi_select blank_fill short_answer essay"`
	Prompt      string       `json:"prompt" validate:"required,max=20000"`
	Explanation *string      `json:"explanation" validate:"omitempty,max=20000"`
	// Points defaults to the section's default marks (or 1) when omitted.
	Points         *float64 `json:"points" validate:"omitempty,gt=0,lte=1000"`
	NegativePoints float64  `json:"negative_points" validate:"gte=0,lte=1000"`

	Options     []OptionInput     `json:"options" validate:"omitempty,max=26,dive"`
	TrueFalse   *TrueFalseInput   `json:"true_false"`
	MultiSelect *MultiSelectInput `json:"multi_select"`
	BlankFill   *BlankFillInput   `json:"blank_fill"`
	ShortAnswer *ShortAnswerInput `json:"short_answer"`
	Essay       *EssayInput       `json:"essay"`
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
	RubricGuidelines *string `json:"rubric_guidelines" validate:"omitempty,max=20000"`
	MinWords         *int    `json:"min_words" validate:"omitempty,gte=0,lte=20000"`
	MaxWords         *int    `json:"max_words" validate:"omitempty,gte=1,lte=20000"`
	MaxLength        *int    `json:"max_length" validate:"omitempty,gte=1,lte=500000"`
	AllowRichText    *bool   `json:"allow_rich_text"`
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
// and applying defaults. Options are ordered as submitted.
func build(in *QuestionInput, sectionType exams.SectionType) *Question {
	q := &Question{
		SectionType:    sectionType,
		Type:           in.Type,
		Prompt:         strings.TrimSpace(in.Prompt),
		Explanation:    trimmedOrNil(in.Explanation),
		Points:         pointsOf(in),
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
			if e.AllowRichText != nil {
				spec.AllowRichText = *e.AllowRichText
			}
		}
		q.Essay = spec

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

// fallbackPoints is used when neither the question nor its section sets marks.
const fallbackPoints = 1.0

// pointsOf returns the question's marks; the service fills in the section
// default before validation, so nil only remains in direct unit use.
func pointsOf(in *QuestionInput) float64 {
	if in.Points == nil {
		return fallbackPoints
	}
	return *in.Points
}

// withDefaultPoints fills in omitted marks from the section default.
func withDefaultPoints(in *QuestionInput, ref *SectionRef) {
	if in.Points != nil {
		return
	}
	p := fallbackPoints
	if ref.DefaultPoints != nil {
		p = *ref.DefaultPoints
	}
	in.Points = &p
}
