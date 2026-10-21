package attempts

import (
	"math"
	"slices"
	"strings"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/questions"
	"github.com/google/uuid"
)

// scoringPolicy is the part of the exam configuration grading depends on.
type scoringPolicy struct {
	NegativeMarking bool
	PenaltyPercent  float64
	MultiSelectMode exams.MultiSelectScoringMode
}

func policyFrom(cfg *exams.ExamConfig) scoringPolicy {
	return scoringPolicy{
		NegativeMarking: cfg.Scoring.NegativeMarkingEnabled,
		PenaltyPercent:  cfg.Scoring.NegativeMarkingPenaltyPercent,
		MultiSelectMode: cfg.Scoring.MultiSelectScoringMode,
	}
}

type gradeResult struct {
	Score float64
	// Correct is nil when the question was left unanswered or needs a human.
	Correct *bool
	// Auto is false for questions that need manual grading.
	Auto bool
}

// grade scores one response against the answer key. points is the value the
// question carried when the paper was built.
func grade(q *questions.Question, points float64, r *Response, p scoringPolicy) gradeResult {
	switch q.Type {
	case questions.TypeShortAnswer, questions.TypeEssay:
		return gradeResult{}
	}
	if !r.answered() {
		return gradeResult{Auto: true}
	}

	penalty := 0.0
	if p.NegativeMarking {
		penalty = q.NegativePoints
		if penalty == 0 {
			penalty = points * p.PenaltyPercent / 100
		}
	}
	binary := func(ok bool) gradeResult {
		if ok {
			return gradeResult{Score: points, Correct: &ok, Auto: true}
		}
		return gradeResult{Score: round2(-penalty), Correct: &ok, Auto: true}
	}

	switch q.Type {
	case questions.TypeMultipleChoice:
		ok := r.SelectedOptionID != nil && slices.ContainsFunc(q.Options, func(o questions.Option) bool {
			return o.IsCorrect && o.ID == *r.SelectedOptionID
		})
		return binary(ok)

	case questions.TypeTrueFalse:
		return binary(r.BooleanResponse != nil && q.TrueFalse != nil && *r.BooleanResponse == q.TrueFalse.CorrectAnswer)

	case questions.TypeBlankFill:
		return binary(r.TextResponse != nil && blankMatches(q.BlankFill, *r.TextResponse))

	case questions.TypeMultiSelect:
		return gradeMultiSelect(q, points, penalty, r.SelectedOptionIDs, p)
	}
	return gradeResult{}
}

// gradeMultiSelect applies the exam's multi-select scoring mode. Option
// weights set each option's share of the credit.
//
//   - all_or_nothing: full points for exactly the correct set, otherwise the
//     wrong-answer penalty.
//   - partial_no_penalty: credit for the share of correct weight selected,
//     but nothing at all if any wrong option is selected.
//   - partial_with_penalty: credit for the share of correct weight selected
//     minus the share of wrong weight selected. Below zero only with
//     negative marking, and never below the wrong-answer penalty.
func gradeMultiSelect(q *questions.Question, points, penalty float64, selected []uuid.UUID, p scoringPolicy) gradeResult {
	var correctW, wrongW, pickedCorrect, pickedWrong float64
	exact := true
	for _, o := range q.Options {
		picked := slices.Contains(selected, o.ID)
		if o.IsCorrect {
			correctW += o.Weight
			if picked {
				pickedCorrect += o.Weight
			}
		} else {
			wrongW += o.Weight
			if picked {
				pickedWrong += o.Weight
			}
		}
		if picked != o.IsCorrect {
			exact = false
		}
	}

	var score float64
	switch p.MultiSelectMode {
	case "all_or_nothing":
		if exact {
			score = points
		} else {
			score = -penalty
		}
	case "partial_no_penalty":
		if pickedWrong == 0 && correctW > 0 {
			score = points * pickedCorrect / correctW
		}
	default: // partial_with_penalty
		share := 0.0
		if correctW > 0 {
			share = pickedCorrect / correctW
		}
		if wrongW > 0 {
			share -= pickedWrong / wrongW
		}
		score = points * share
		if score < 0 {
			score = max(score, -penalty)
		}
	}
	score = round2(score)
	return gradeResult{Score: score, Correct: &exact, Auto: true}
}

func blankMatches(key *questions.BlankFillSpec, answer string) bool {
	if key == nil {
		return false
	}
	norm := func(s string) string {
		s = strings.Join(strings.Fields(s), " ")
		if !key.CaseSensitive {
			s = strings.ToLower(s)
		}
		return s
	}
	got := norm(answer)
	if got == "" {
		return false
	}
	if got == norm(key.CorrectAnswer) {
		return true
	}
	return slices.ContainsFunc(key.AlternateAnswers, func(a string) bool { return got == norm(a) })
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
