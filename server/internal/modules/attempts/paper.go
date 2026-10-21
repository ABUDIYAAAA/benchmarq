package attempts

import (
	crand "crypto/rand"
	"encoding/binary"
	"hash/fnv"
	"math/rand/v2"
	"slices"

	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/google/uuid"
)

// A paper is the frozen set and order of sections, questions and options one
// attempt is served. It is built once, when the attempt starts, from a seed.

type paperSection struct {
	ID               uuid.UUID
	Rules            exams.EffectiveRules
	TimeLimitSeconds *int
	Questions        []paperSource // authored order
}

type paperSource struct {
	ID      uuid.UUID
	Points  float64
	Options []optionSource // authored order
}

type optionSource struct {
	ID     uuid.UUID
	Pinned bool
}

// attemptSeed picks the randomization seed. A deterministic seed gives the
// candidate the same paper on every attempt; otherwise each attempt differs.
func attemptSeed(deterministic bool, examID, userID uuid.UUID) int64 {
	if deterministic {
		h := fnv.New64a()
		h.Write(examID[:])
		h.Write(userID[:])
		return int64(h.Sum64())
	}
	var b [8]byte
	_, _ = crand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]))
}

// buildPaper draws and orders questions for every section. Sections must be
// in authored order; with shuffleSections the candidate's order is shuffled.
func buildPaper(seed int64, sections []paperSection, shuffleSections bool) ([]SectionState, []PaperQuestion) {
	rng := rand.New(rand.NewPCG(uint64(seed), 0x9e3779b97f4a7c15))

	order := make([]int, len(sections))
	for i := range order {
		order[i] = i
	}
	if shuffleSections {
		rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	}

	states := make([]SectionState, len(sections))
	var paper []PaperQuestion
	for pos, idx := range order {
		s := sections[idx]
		states[pos] = SectionState{
			SectionID:        s.ID,
			Position:         pos,
			Status:           SectionNotStarted,
			TimeLimitSeconds: s.TimeLimitSeconds,
		}
		for qpos, q := range drawQuestions(rng, s) {
			pq := PaperQuestion{
				QuestionID:  q.ID,
				SectionID:   s.ID,
				Position:    qpos,
				Points:      q.Points,
				OptionOrder: orderOptions(rng, q.Options, s.Rules.ShuffleOptions),
			}
			paper = append(paper, pq)
		}
	}
	return states, paper
}

// drawQuestions samples the section's pool, if it has one, and orders the
// result. Without shuffling, drawn questions keep their authored order.
func drawQuestions(rng *rand.Rand, s paperSection) []paperSource {
	n := len(s.Questions)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	if s.Rules.ShuffleQuestions || (s.Rules.QuestionPickCount != nil && *s.Rules.QuestionPickCount < n) {
		rng.Shuffle(n, func(i, j int) { idx[i], idx[j] = idx[j], idx[i] })
	}
	if pick := s.Rules.QuestionPickCount; pick != nil && *pick < n {
		idx = idx[:*pick]
	}
	if !s.Rules.ShuffleQuestions {
		slices.Sort(idx)
	}

	out := make([]paperSource, len(idx))
	for i, j := range idx {
		out[i] = s.Questions[j]
	}
	return out
}

// orderOptions shuffles options when asked, leaving pinned options (such as
// "None of the above") at their authored positions.
func orderOptions(rng *rand.Rand, options []optionSource, shuffle bool) []uuid.UUID {
	out := make([]uuid.UUID, len(options))
	var free []int
	for i, o := range options {
		out[i] = o.ID
		if !o.Pinned {
			free = append(free, i)
		}
	}
	if !shuffle || len(free) < 2 {
		return out
	}

	ids := make([]uuid.UUID, len(free))
	for i, slot := range free {
		ids[i] = out[slot]
	}
	rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	for i, slot := range free {
		out[slot] = ids[i]
	}
	return out
}
