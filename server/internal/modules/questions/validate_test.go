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
			section: exams.SectionCoding,
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
		{
			name:    "coding requires spec",
			section: exams.SectionCoding,
			in:      QuestionInput{Type: TypeCoding, Prompt: "Reverse a list", Points: 10},
			wantErr: "coding",
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
		Type: TypeCoding, Prompt: " Sum two numbers ", Points: 5,
		Coding: &CodingInput{TestCases: []TestCaseInput{{Input: "1 2", ExpectedOutput: "3", IsHidden: ptr(false)}, {Input: "2 2", ExpectedOutput: "4"}}},
	}
	q := build(&in, exams.SectionCoding)
	if q.Prompt != "Sum two numbers" {
		t.Errorf("prompt not trimmed: %q", q.Prompt)
	}
	c := q.Coding
	if c.TimeLimitMs != defaultCodingTimeLimitMs || c.MemoryLimitMb != defaultCodingMemoryLimitMb {
		t.Errorf("coding defaults not applied: %+v", c)
	}
	if c.TestCases[0].IsHidden || !c.TestCases[1].IsHidden || c.TestCases[1].Points != 1 || c.TestCases[1].Position != 1 {
		t.Errorf("test case defaults wrong: %+v", c.TestCases)
	}

	essay := build(&QuestionInput{Type: TypeEssay, Prompt: "x", Points: 1}, exams.SectionSubjective)
	if !essay.Essay.AllowRichText {
		t.Error("essay should allow rich text by default")
	}
}
