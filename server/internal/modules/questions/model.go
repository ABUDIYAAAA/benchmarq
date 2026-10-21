package questions

import (
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/google/uuid"
)

type QuestionType string

const (
	TypeMultipleChoice QuestionType = "multiple_choice"
	TypeTrueFalse      QuestionType = "true_false"
	TypeMultiSelect    QuestionType = "multi_select"
	TypeBlankFill      QuestionType = "blank_fill"
	TypeShortAnswer    QuestionType = "short_answer"
	TypeEssay          QuestionType = "essay"
)

// family maps each question type to the only section type that may hold it.
// The same rule is enforced by a CHECK constraint in the database.
var family = map[QuestionType]exams.SectionType{
	TypeMultipleChoice: exams.SectionObjective,
	TypeTrueFalse:      exams.SectionObjective,
	TypeMultiSelect:    exams.SectionObjective,
	TypeBlankFill:      exams.SectionObjective,
	TypeShortAnswer:    exams.SectionSubjective,
	TypeEssay:          exams.SectionSubjective,
}

func (t QuestionType) usesOptions() bool {
	return t == TypeMultipleChoice || t == TypeMultiSelect
}

// Question is the authoring view of a question, including answer keys.
// Exactly one type-specific block is set, matching Type.
type Question struct {
	ID             uuid.UUID         `json:"id"`
	SectionID      uuid.UUID         `json:"section_id"`
	SectionType    exams.SectionType `json:"section_type"`
	Type           QuestionType      `json:"type"`
	Prompt         string            `json:"prompt"`
	Explanation    *string           `json:"explanation,omitempty"`
	Points         float64           `json:"points"`
	NegativePoints float64           `json:"negative_points"`
	Position       int               `json:"position"`

	Options     []Option         `json:"options,omitempty"`
	TrueFalse   *TrueFalseSpec   `json:"true_false,omitempty"`
	MultiSelect *MultiSelectSpec `json:"multi_select,omitempty"`
	BlankFill   *BlankFillSpec   `json:"blank_fill,omitempty"`
	ShortAnswer *ShortAnswerSpec `json:"short_answer,omitempty"`
	Essay       *EssaySpec       `json:"essay,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Option struct {
	ID        uuid.UUID `json:"id"`
	Content   string    `json:"content"`
	Position  int       `json:"position"`
	IsCorrect bool      `json:"is_correct"`
	// IsPinned keeps the option at its authored position when options are
	// shuffled (e.g. "None of the above").
	IsPinned bool `json:"is_pinned"`
	// Weight is the option's share of credit under partial multi-select scoring.
	Weight float64 `json:"weight"`
}

type TrueFalseSpec struct {
	CorrectAnswer bool `json:"correct_answer"`
}

type MultiSelectSpec struct {
	MinChoicesRequired int  `json:"min_choices_required"`
	MaxChoicesAllowed  *int `json:"max_choices_allowed,omitempty"`
}

type BlankFillSpec struct {
	CorrectAnswer    string   `json:"correct_answer"`
	CaseSensitive    bool     `json:"case_sensitive"`
	AlternateAnswers []string `json:"alternate_answers"`
}

type ShortAnswerSpec struct {
	SampleAnswer   *string  `json:"sample_answer,omitempty"`
	MaxLength      *int     `json:"max_length,omitempty"`
	MaxWords       *int     `json:"max_words,omitempty"`
	RubricKeywords []string `json:"rubric_keywords"`
}

type EssaySpec struct {
	RubricGuidelines *string `json:"rubric_guidelines,omitempty"`
	MinWords         *int    `json:"min_words,omitempty"`
	MaxWords         *int    `json:"max_words,omitempty"`
	MaxLength        *int    `json:"max_length,omitempty"`
	AllowRichText    bool    `json:"allow_rich_text"`
}
