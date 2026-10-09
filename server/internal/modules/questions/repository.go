package questions

import (
	"context"
	"errors"
	"fmt"

	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSectionNotFound  = errors.New("section not found")
	ErrQuestionNotFound = errors.New("question not found")
	ErrUnknownLanguage  = errors.New("coding language not found")
)

// SectionRef is the minimal context needed to author questions in a section.
type SectionRef struct {
	ExamStatus  exams.ExamStatus
	SectionType exams.SectionType
}

type Repository interface {
	// LockSection resolves the section within the org's exam and locks the
	// exam row, serializing writes with other authoring and publish requests.
	LockSection(ctx context.Context, orgID, examID, sectionID uuid.UUID) (*SectionRef, error)
	GetSection(ctx context.Context, orgID, examID, sectionID uuid.UUID) (*SectionRef, error)

	List(ctx context.Context, sectionID uuid.UUID) ([]Question, error)
	Get(ctx context.Context, sectionID, questionID uuid.UUID) (*Question, error)
	ListIDs(ctx context.Context, sectionID uuid.UUID) ([]uuid.UUID, error)
	Insert(ctx context.Context, q *Question) error
	Replace(ctx context.Context, q *Question) error
	Delete(ctx context.Context, sectionID, questionID uuid.UUID) (position int, err error)
	ShiftPositions(ctx context.Context, sectionID uuid.UUID, from, delta int) error
	SetOrder(ctx context.Context, sectionID uuid.UUID, ordered []uuid.UUID) error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) LockSection(ctx context.Context, orgID, examID, sectionID uuid.UUID) (*SectionRef, error) {
	return r.section(ctx, orgID, examID, sectionID, true)
}

func (r *postgresRepository) GetSection(ctx context.Context, orgID, examID, sectionID uuid.UUID) (*SectionRef, error) {
	return r.section(ctx, orgID, examID, sectionID, false)
}

func (r *postgresRepository) section(ctx context.Context, orgID, examID, sectionID uuid.UUID, lock bool) (*SectionRef, error) {
	query := `
		SELECT e.status, s.section_type
		FROM exams e
		JOIN exam_sections s ON s.exam_id = e.id
		WHERE e.id = $1 AND e.org_id = $2 AND e.deleted_at IS NULL AND s.id = $3
	`
	if lock {
		query += ` FOR UPDATE OF e`
	}
	var ref SectionRef
	if err := database.Conn(ctx, r.db).QueryRow(ctx, query, examID, orgID, sectionID).Scan(&ref.ExamStatus, &ref.SectionType); err != nil {
		if database.IsNoRows(err) {
			return nil, ErrSectionNotFound
		}
		return nil, fmt.Errorf("resolve section: %w", err)
	}
	return &ref, nil
}

// ---------------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------------

const questionSelect = `
	SELECT id, section_id, section_type, type, prompt, explanation,
	       points::float8, negative_points::float8, position, created_at, updated_at
	FROM questions
`

func (r *postgresRepository) List(ctx context.Context, sectionID uuid.UUID) ([]Question, error) {
	return r.load(ctx, questionSelect+` WHERE section_id = $1 ORDER BY position`, sectionID)
}

func (r *postgresRepository) Get(ctx context.Context, sectionID, questionID uuid.UUID) (*Question, error) {
	qs, err := r.load(ctx, questionSelect+` WHERE section_id = $1 AND id = $2`, sectionID, questionID)
	if err != nil {
		return nil, err
	}
	if len(qs) == 0 {
		return nil, ErrQuestionNotFound
	}
	return &qs[0], nil
}

func (r *postgresRepository) ListIDs(ctx context.Context, sectionID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := database.Conn(ctx, r.db).Query(ctx,
		`SELECT id FROM questions WHERE section_id = $1 ORDER BY position`, sectionID)
	if err != nil {
		return nil, fmt.Errorf("list question ids: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, fmt.Errorf("scan question ids: %w", err)
	}
	return ids, nil
}

// load fetches questions and then all of their child rows in a single
// pipelined batch (one round trip), avoiding N+1 queries.
func (r *postgresRepository) load(ctx context.Context, query string, args ...any) ([]Question, error) {
	conn := database.Conn(ctx, r.db)
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select questions: %w", err)
	}
	questions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Question, error) {
		var q Question
		err := row.Scan(&q.ID, &q.SectionID, &q.SectionType, &q.Type, &q.Prompt, &q.Explanation,
			&q.Points, &q.NegativePoints, &q.Position, &q.CreatedAt, &q.UpdatedAt)
		return q, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan questions: %w", err)
	}
	if len(questions) == 0 {
		return questions, nil
	}

	ids := make([]uuid.UUID, len(questions))
	byID := make(map[uuid.UUID]*Question, len(questions))
	for i := range questions {
		ids[i] = questions[i].ID
		byID[questions[i].ID] = &questions[i]
	}

	batch := &pgx.Batch{}
	batch.Queue(`
		SELECT question_id, id, content, position, is_correct, is_pinned, weight::float8
		FROM question_options WHERE question_id = ANY($1) ORDER BY question_id, position
	`, ids).Query(func(rows pgx.Rows) error {
		return forEach(rows, func() error {
			var qid uuid.UUID
			var o Option
			if err := rows.Scan(&qid, &o.ID, &o.Content, &o.Position, &o.IsCorrect, &o.IsPinned, &o.Weight); err != nil {
				return err
			}
			byID[qid].Options = append(byID[qid].Options, o)
			return nil
		})
	})
	batch.Queue(`SELECT question_id, correct_answer FROM true_false_questions WHERE question_id = ANY($1)`, ids).
		Query(func(rows pgx.Rows) error {
			return forEach(rows, func() error {
				var qid uuid.UUID
				var s TrueFalseSpec
				if err := rows.Scan(&qid, &s.CorrectAnswer); err != nil {
					return err
				}
				byID[qid].TrueFalse = &s
				return nil
			})
		})
	batch.Queue(`SELECT question_id, min_choices_required, max_choices_allowed FROM multi_select_questions WHERE question_id = ANY($1)`, ids).
		Query(func(rows pgx.Rows) error {
			return forEach(rows, func() error {
				var qid uuid.UUID
				var s MultiSelectSpec
				if err := rows.Scan(&qid, &s.MinChoicesRequired, &s.MaxChoicesAllowed); err != nil {
					return err
				}
				byID[qid].MultiSelect = &s
				return nil
			})
		})
	batch.Queue(`SELECT question_id, correct_answer, case_sensitive, alternate_answers FROM blank_fill_questions WHERE question_id = ANY($1)`, ids).
		Query(func(rows pgx.Rows) error {
			return forEach(rows, func() error {
				var qid uuid.UUID
				var s BlankFillSpec
				if err := rows.Scan(&qid, &s.CorrectAnswer, &s.CaseSensitive, &s.AlternateAnswers); err != nil {
					return err
				}
				byID[qid].BlankFill = &s
				return nil
			})
		})
	batch.Queue(`SELECT question_id, sample_answer, max_length, max_words, rubric_keywords FROM short_answer_questions WHERE question_id = ANY($1)`, ids).
		Query(func(rows pgx.Rows) error {
			return forEach(rows, func() error {
				var qid uuid.UUID
				var s ShortAnswerSpec
				if err := rows.Scan(&qid, &s.SampleAnswer, &s.MaxLength, &s.MaxWords, &s.RubricKeywords); err != nil {
					return err
				}
				byID[qid].ShortAnswer = &s
				return nil
			})
		})
	batch.Queue(`
		SELECT question_id, rubric_guidelines, min_words, max_words, max_length, allow_rich_text, allow_file_attachments
		FROM essay_questions WHERE question_id = ANY($1)
	`, ids).Query(func(rows pgx.Rows) error {
		return forEach(rows, func() error {
			var qid uuid.UUID
			var s EssaySpec
			if err := rows.Scan(&qid, &s.RubricGuidelines, &s.MinWords, &s.MaxWords, &s.MaxLength, &s.AllowRichText, &s.AllowFileAttachments); err != nil {
				return err
			}
			byID[qid].Essay = &s
			return nil
		})
	})
	batch.Queue(`
		SELECT question_id, default_language_id, starter_code, solution_code, driver_code, time_limit_ms, memory_limit_mb
		FROM coding_questions WHERE question_id = ANY($1)
	`, ids).Query(func(rows pgx.Rows) error {
		return forEach(rows, func() error {
			var qid uuid.UUID
			s := CodingSpec{TestCases: []TestCase{}}
			if err := rows.Scan(&qid, &s.DefaultLanguageID, &s.StarterCode, &s.SolutionCode, &s.DriverCode, &s.TimeLimitMs, &s.MemoryLimitMb); err != nil {
				return err
			}
			byID[qid].Coding = &s
			return nil
		})
	})
	batch.Queue(`
		SELECT question_id, id, input, expected_output, points::float8, position, is_hidden, time_limit_override_ms
		FROM coding_test_cases WHERE question_id = ANY($1) ORDER BY question_id, position
	`, ids).Query(func(rows pgx.Rows) error {
		return forEach(rows, func() error {
			var qid uuid.UUID
			var tc TestCase
			if err := rows.Scan(&qid, &tc.ID, &tc.Input, &tc.ExpectedOutput, &tc.Points, &tc.Position, &tc.IsHidden, &tc.TimeLimitOverrideMs); err != nil {
				return err
			}
			// coding_questions is queued first, so the spec already exists.
			if c := byID[qid].Coding; c != nil {
				c.TestCases = append(c.TestCases, tc)
			}
			return nil
		})
	})

	if err := conn.SendBatch(ctx, batch).Close(); err != nil {
		return nil, fmt.Errorf("load question details: %w", err)
	}
	return questions, nil
}

func forEach(rows pgx.Rows, fn func() error) error {
	defer rows.Close()
	for rows.Next() {
		if err := fn(); err != nil {
			return err
		}
	}
	return rows.Err()
}

// ---------------------------------------------------------------------------
// Writes
// ---------------------------------------------------------------------------

func (r *postgresRepository) Insert(ctx context.Context, q *Question) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	err := database.Conn(ctx, r.db).QueryRow(ctx, `
		INSERT INTO questions (id, section_id, section_type, type, prompt, explanation, points, negative_points, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`, q.ID, q.SectionID, q.SectionType, q.Type, q.Prompt, q.Explanation, q.Points, q.NegativePoints, q.Position,
	).Scan(&q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert question: %w", err)
	}
	return r.insertDetails(ctx, q)
}

// Replace overwrites a question in place, keeping its ID and position. Child
// rows are rebuilt from scratch, which also handles changes of type.
func (r *postgresRepository) Replace(ctx context.Context, q *Question) error {
	conn := database.Conn(ctx, r.db)
	err := conn.QueryRow(ctx, `
		UPDATE questions
		SET type = $3, prompt = $4, explanation = $5, points = $6, negative_points = $7, updated_at = now()
		WHERE id = $1 AND section_id = $2
		RETURNING position, created_at, updated_at
	`, q.ID, q.SectionID, q.Type, q.Prompt, q.Explanation, q.Points, q.NegativePoints,
	).Scan(&q.Position, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		if database.IsNoRows(err) {
			return ErrQuestionNotFound
		}
		return fmt.Errorf("update question: %w", err)
	}

	// Order matters: answer-key rows reference options, so they go first.
	batch := &pgx.Batch{}
	for _, table := range []string{
		"multiple_choice_questions",
		"multi_select_questions", // cascades to multi_select_correct_options
		"true_false_questions",
		"blank_fill_questions",
		"short_answer_questions",
		"essay_questions",
		"coding_questions", // cascades to coding_test_cases
		"question_options",
	} {
		batch.Queue(`DELETE FROM `+table+` WHERE question_id = $1`, q.ID)
	}
	if err := conn.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("clear question details: %w", err)
	}
	return r.insertDetails(ctx, q)
}

// insertDetails writes options and the type-specific rows in one batch.
func (r *postgresRepository) insertDetails(ctx context.Context, q *Question) error {
	batch := &pgx.Batch{}

	for _, o := range q.Options {
		batch.Queue(`
			INSERT INTO question_options (id, question_id, content, position, is_correct, is_pinned, weight)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, o.ID, q.ID, o.Content, o.Position, o.IsCorrect, o.IsPinned, o.Weight)
	}

	switch q.Type {
	case TypeMultipleChoice:
		for _, o := range q.Options {
			if o.IsCorrect {
				batch.Queue(`INSERT INTO multiple_choice_questions (question_id, correct_option_id) VALUES ($1, $2)`, q.ID, o.ID)
				break
			}
		}

	case TypeMultiSelect:
		ms := q.MultiSelect
		batch.Queue(`INSERT INTO multi_select_questions (question_id, min_choices_required, max_choices_allowed) VALUES ($1, $2, $3)`,
			q.ID, ms.MinChoicesRequired, ms.MaxChoicesAllowed)
		for _, o := range q.Options {
			if o.IsCorrect {
				batch.Queue(`INSERT INTO multi_select_correct_options (question_id, option_id) VALUES ($1, $2)`, q.ID, o.ID)
			}
		}

	case TypeTrueFalse:
		batch.Queue(`INSERT INTO true_false_questions (question_id, correct_answer) VALUES ($1, $2)`, q.ID, q.TrueFalse.CorrectAnswer)

	case TypeBlankFill:
		bf := q.BlankFill
		batch.Queue(`INSERT INTO blank_fill_questions (question_id, correct_answer, case_sensitive, alternate_answers) VALUES ($1, $2, $3, $4)`,
			q.ID, bf.CorrectAnswer, bf.CaseSensitive, bf.AlternateAnswers)

	case TypeShortAnswer:
		sa := q.ShortAnswer
		batch.Queue(`INSERT INTO short_answer_questions (question_id, sample_answer, max_length, max_words, rubric_keywords) VALUES ($1, $2, $3, $4, $5)`,
			q.ID, sa.SampleAnswer, sa.MaxLength, sa.MaxWords, sa.RubricKeywords)

	case TypeEssay:
		e := q.Essay
		batch.Queue(`
			INSERT INTO essay_questions (question_id, rubric_guidelines, min_words, max_words, max_length, allow_rich_text, allow_file_attachments)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, q.ID, e.RubricGuidelines, e.MinWords, e.MaxWords, e.MaxLength, e.AllowRichText, e.AllowFileAttachments)

	case TypeCoding:
		c := q.Coding
		batch.Queue(`
			INSERT INTO coding_questions (question_id, default_language_id, starter_code, solution_code, driver_code, time_limit_ms, memory_limit_mb)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, q.ID, c.DefaultLanguageID, c.StarterCode, c.SolutionCode, c.DriverCode, c.TimeLimitMs, c.MemoryLimitMb)
		for _, tc := range c.TestCases {
			batch.Queue(`
				INSERT INTO coding_test_cases (id, question_id, input, expected_output, points, position, is_hidden, time_limit_override_ms)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`, tc.ID, q.ID, tc.Input, tc.ExpectedOutput, tc.Points, tc.Position, tc.IsHidden, tc.TimeLimitOverrideMs)
		}
	}

	if batch.Len() == 0 {
		return nil
	}
	if err := database.Conn(ctx, r.db).SendBatch(ctx, batch).Close(); err != nil {
		if database.IsForeignKeyViolation(err, "coding_questions_default_language_id_fkey") {
			return ErrUnknownLanguage
		}
		return fmt.Errorf("insert question details: %w", err)
	}
	return nil
}

func (r *postgresRepository) Delete(ctx context.Context, sectionID, questionID uuid.UUID) (int, error) {
	var position int
	err := database.Conn(ctx, r.db).QueryRow(ctx,
		`DELETE FROM questions WHERE id = $1 AND section_id = $2 RETURNING position`, questionID, sectionID,
	).Scan(&position)
	if err != nil {
		if database.IsNoRows(err) {
			return 0, ErrQuestionNotFound
		}
		return 0, fmt.Errorf("delete question: %w", err)
	}
	return position, nil
}

// ShiftPositions relies on the DEFERRABLE unique (section_id, position)
// constraint, which is checked at the end of the statement.
func (r *postgresRepository) ShiftPositions(ctx context.Context, sectionID uuid.UUID, from, delta int) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx,
		`UPDATE questions SET position = position + $3 WHERE section_id = $1 AND position >= $2`,
		sectionID, from, delta)
	if err != nil {
		return fmt.Errorf("shift questions: %w", err)
	}
	return nil
}

func (r *postgresRepository) SetOrder(ctx context.Context, sectionID uuid.UUID, ordered []uuid.UUID) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx, `
		UPDATE questions q
		SET position = o.ord - 1, updated_at = now()
		FROM unnest($2::uuid[]) WITH ORDINALITY AS o(id, ord)
		WHERE q.id = o.id AND q.section_id = $1 AND q.position <> o.ord - 1
	`, sectionID, ordered)
	if err != nil {
		return fmt.Errorf("reorder questions: %w", err)
	}
	return nil
}
