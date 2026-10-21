package questions

import (
	"testing"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
)

func opts(correct ...bool) []OptionInput {
	out := make([]OptionInput, len(correct))
	for i, c := range correct {
		out[i] = OptionInput{Content: string(rune('A' + i)), IsCorrect: c}
	}
	return out
}

func ptr[T any](v T) *T { return &v }

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		section exams.SectionType
		in      QuestionInput
		wantErr string // expected error key, "" for valid
	}{
		{
			name:    "valid multiple choice",
			section: exams.SectionObjective,
			in:      QuestionInput{Type: TypeMultipleChoice, Prompt: "2+2?", Points: 1, Options: opts(false, true, false)},
		},
		{
			name:    "multiple choice needs exactly one correct",
			section: exams.SectionObjective,
			in:      QuestionInput{Type: TypeMultipleChoice, Prompt: "?", Points: 1, Options: opts(true, true)},
			wantErr: "options",
		},
		{
			name:    "too few options",
			section: exams.SectionObjective,
			in:      QuestionInput{Type: TypeMultipleChoice, Prompt: "?", Points: 1, Options: opts(true)},
			wantErr: "options",
		},
		{
			name:    "duplicate option text ignores case and spacing",
			section: exams.SectionObjective,
			in: QuestionInput{Type: TypeMultipleChoice, Prompt: "?", Points: 1, Options: []OptionInput{
				{Content: "Paris", IsCorrect: true}, {Content: "  paris "},
			}},
			wantErr: "options[1].content",
		},
		{
			name:    "wrong section family",
			section: exams.SectionSubjective,
			in:      QuestionInput{Type: TypeTrueFalse, Prompt: "?", Points: 1, TrueFalse: &TrueFalseInput{CorrectAnswer: ptr(true)}},
			wantErr: "type",
		},
		{
			name:    "foreign block rejected",
			section: exams.SectionObjective,
			in: QuestionInput{Type: TypeTrueFalse, Prompt: "?", Points: 1,
				TrueFalse: &TrueFalseInput{CorrectAnswer: ptr(false)}, Options: opts(true, false)},
			wantErr: "options",
		},
		{
			name:    "negative points capped by points",
			section: exams.SectionObjective,
			in:      QuestionInput{Type: TypeTrueFalse, Prompt: "?", Points: 1, NegativePoints: 2, TrueFalse: &TrueFalseInput{CorrectAnswer: ptr(true)}},
			wantErr: "negative_points",
		},
		{
			name:    "multi-select more correct answers than selectable",
			section: exams.SectionObjective,
			in: QuestionInput{Type: TypeMultiSelect, Prompt: "?", Points: 2, Options: opts(true, true, true, false),
				MultiSelect: &MultiSelectInput{MaxChoicesAllowed: ptr(2)}},
			wantErr: "multi_select.max_choices_allowed",
		},
		{
			name:    "multi-select forces a wrong pick",
			section: exams.SectionObjective,
			in: QuestionInput{Type: TypeMultiSelect, Prompt: "?", Points: 2, Options: opts(true, false, false),
				MultiSelect: &MultiSelectInput{MinChoicesRequired: ptr(2)}},
			wantErr: "multi_select.min_choices_required",
		},
		{
			name:    "valid multi-select with defaults",
			section: exams.SectionObjective,
			in:      QuestionInput{Type: TypeMultiSelect, Prompt: "?", Points: 2, Options: opts(true, true, false)},
		},
		{
			name:    "blank fill duplicate alternate (case-insensitive)",
			section: exams.SectionObjective,
			in: QuestionInput{Type: TypeBlankFill, Prompt: "Capital of France?", Points: 1,
				BlankFill: &BlankFillInput{CorrectAnswer: "Paris", AlternateAnswers: []string{"PARIS"}}},
			wantErr: "blank_fill.alternate_answers[0]",
		},
		{
			name:    "blank fill case-sensitive alternates are distinct",
			section: exams.SectionObjective,
			in: QuestionInput{Type: TypeBlankFill, Prompt: "?", Points: 1,
				BlankFill: &BlankFillInput{CorrectAnswer: "Go", CaseSensitive: true, AlternateAnswers: []string{"GO"}}},
		},
		{
			name:    "essay word bounds",
			section: exams.SectionSubjective,
			in:      QuestionInput{Type: TypeEssay, Prompt: "Discuss.", Points: 10, Essay: &EssayInput{MinWords: ptr(500), MaxWords: ptr(100)}},
			wantErr: "essay.min_words",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := Validate(tc.section, &tc.in)
			if tc.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("expected valid, got %v", errs)
				}
				return
			}
			if _, ok := errs[tc.wantErr]; !ok {
				t.Fatalf("expected error on %q, got %v", tc.wantErr, errs)
			}
		})
	}
}

func TestBuildAppliesDefaults(t *testing.T) {
	in := QuestionInput{
		Type: TypeMultiSelect, Prompt: " Pick primes ", Points: 5,
		Options: []OptionInput{{Content: " 2 ", IsCorrect: true}, {Content: "3", IsCorrect: true, Weight: ptr(2.0)}, {Content: "4"}},
	}
	q := build(&in, exams.SectionObjective)
	if q.Prompt != "Pick primes" {
		t.Errorf("prompt not trimmed: %q", q.Prompt)
	}
	if q.MultiSelect.MinChoicesRequired != 1 || q.MultiSelect.MaxChoicesAllowed != nil {
		t.Errorf("multi-select defaults not applied: %+v", q.MultiSelect)
	}
	if o := q.Options; o[0].Content != "2" || o[0].Weight != 1 || o[1].Weight != 2 || o[2].Position != 2 {
		t.Errorf("option defaults wrong: %+v", o)
	}

	essay := build(&QuestionInput{Type: TypeEssay, Prompt: "x", Points: 1}, exams.SectionSubjective)
	if !essay.Essay.AllowRichText {
		t.Error("essay should allow rich text by default")
	}
}
