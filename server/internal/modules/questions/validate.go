package questions

import (
	"fmt"
	"strings"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
)

const (
	minOptions = 2
	maxOptions = 26
)

// Validate applies the semantic rules for a question in a section of the
// given type. Struct tags have already checked field-level bounds. It returns
// a path -> message map, empty when the question is valid.
func Validate(sectionType exams.SectionType, in *QuestionInput) map[string]string {
	errs := make(map[string]string)

	if want, ok := family[in.Type]; ok && want != sectionType {
		errs["type"] = fmt.Sprintf("%s questions belong in %s sections, not %s", in.Type, want, sectionType)
	}
	if strings.TrimSpace(in.Prompt) == "" {
		errs["prompt"] = "prompt cannot be blank"
	}
	if in.NegativePoints > in.Points {
		errs["negative_points"] = "negative_points cannot exceed points"
	}

	rejectForeignBlocks(in, errs)

	switch in.Type {
	case TypeMultipleChoice:
		validateOptions(in.Options, errs)
		if n := countCorrect(in.Options); len(in.Options) > 0 && n != 1 {
			errs["options"] = fmt.Sprintf("multiple choice questions need exactly one correct option (found %d)", n)
		}

	case TypeMultiSelect:
		validateOptions(in.Options, errs)
		validateMultiSelect(in, errs)

	case TypeTrueFalse:
		if in.TrueFalse == nil {
			errs["true_false"] = "true_false.correct_answer is required"
		}

	case TypeBlankFill:
		if in.BlankFill == nil {
			errs["blank_fill"] = "blank_fill is required"
		} else {
			validateBlankFill(in.BlankFill, errs)
		}

	case TypeEssay:
		if e := in.Essay; e != nil && e.MinWords != nil && e.MaxWords != nil && *e.MinWords > *e.MaxWords {
			errs["essay.min_words"] = "min_words cannot exceed max_words"
		}

	case TypeCoding:
		if in.Coding == nil {
			errs["coding"] = "coding is required"
		}
	}
	return errs
}

// rejectForeignBlocks reports type-specific blocks that do not match the
// question type, so a client cannot silently send ignored data.
func rejectForeignBlocks(in *QuestionInput, errs map[string]string) {
	blocks := []struct {
		name    string
		present bool
		allowed bool
	}{
		{"options", len(in.Options) > 0, in.Type.usesOptions()},
		{"true_false", in.TrueFalse != nil, in.Type == TypeTrueFalse},
		{"multi_select", in.MultiSelect != nil, in.Type == TypeMultiSelect},
		{"blank_fill", in.BlankFill != nil, in.Type == TypeBlankFill},
		{"short_answer", in.ShortAnswer != nil, in.Type == TypeShortAnswer},
		{"essay", in.Essay != nil, in.Type == TypeEssay},
		{"coding", in.Coding != nil, in.Type == TypeCoding},
	}
	for _, b := range blocks {
		if b.present && !b.allowed {
			errs[b.name] = fmt.Sprintf("%s is not allowed for %s questions", b.name, in.Type)
		}
	}
}

func validateOptions(options []OptionInput, errs map[string]string) {
	if len(options) < minOptions || len(options) > maxOptions {
		errs["options"] = fmt.Sprintf("provide between %d and %d options", minOptions, maxOptions)
		return
	}
	seen := make(map[string]int, len(options))
	for i, o := range options {
		key := normalize(o.Content)
		if key == "" {
			errs[fmt.Sprintf("options[%d].content", i)] = "option content cannot be blank"
			continue
		}
		if first, dup := seen[key]; dup {
			errs[fmt.Sprintf("options[%d].content", i)] = fmt.Sprintf("duplicates option %d", first+1)
			continue
		}
		seen[key] = i
	}
}

func validateMultiSelect(in *QuestionInput, errs map[string]string) {
	if len(in.Options) == 0 {
		return
	}
	correct := countCorrect(in.Options)
	if correct == 0 {
		errs["options"] = "multi-select questions need at least one correct option"
		return
	}

	minChoices, maxChoices := 1, len(in.Options)
	if ms := in.MultiSelect; ms != nil {
		if ms.MinChoicesRequired != nil {
			minChoices = *ms.MinChoicesRequired
		}
		if ms.MaxChoicesAllowed != nil {
			maxChoices = *ms.MaxChoicesAllowed
		}
	}
	switch {
	case minChoices > maxChoices:
		errs["multi_select.min_choices_required"] = "min_choices_required cannot exceed max_choices_allowed"
	case maxChoices > len(in.Options):
		errs["multi_select.max_choices_allowed"] = "max_choices_allowed cannot exceed the number of options"
	case correct > maxChoices:
		// A candidate could never select the full correct answer.
		errs["multi_select.max_choices_allowed"] = fmt.Sprintf("there are %d correct options but only %d may be selected", correct, maxChoices)
	case correct < minChoices:
		// A candidate would be forced to select a wrong option.
		errs["multi_select.min_choices_required"] = fmt.Sprintf("candidates must select %d options but only %d are correct", minChoices, correct)
	}
}

func validateBlankFill(bf *BlankFillInput, errs map[string]string) {
	key := func(s string) string {
		s = strings.TrimSpace(s)
		if !bf.CaseSensitive {
			s = strings.ToLower(s)
		}
		return s
	}
	if key(bf.CorrectAnswer) == "" {
		errs["blank_fill.correct_answer"] = "correct_answer cannot be blank"
		return
	}
	seen := map[string]bool{key(bf.CorrectAnswer): true}
	for i, alt := range bf.AlternateAnswers {
		k := key(alt)
		if seen[k] {
			errs[fmt.Sprintf("blank_fill.alternate_answers[%d]", i)] = "duplicates another accepted answer"
		}
		seen[k] = true
	}
}

func countCorrect(options []OptionInput) int {
	n := 0
	for _, o := range options {
		if o.IsCorrect {
			n++
		}
	}
	return n
}

func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
