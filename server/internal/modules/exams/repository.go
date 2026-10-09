package exams

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrExamNotFound    = errors.New("exam not found")
	ErrExamCodeTaken   = errors.New("exam code already in use")
	ErrSectionNotFound = errors.New("section not found")
)

type Repository interface {
	CreateExam(ctx context.Context, exam *Exam) error
	GetExam(ctx context.Context, orgID, examID uuid.UUID) (*Exam, error)
	// LockExam loads the exam and holds a row lock until the transaction ends.
	// Every authoring write takes this lock, which serializes edits to one exam
	// and stops a publish from racing with a concurrent edit.
	LockExam(ctx context.Context, orgID, examID uuid.UUID) (*Exam, error)
	UpdateExam(ctx context.Context, exam *Exam) error
	SoftDeleteExam(ctx context.Context, examID uuid.UUID) error
	ListExams(ctx context.Context, orgID uuid.UUID, f ExamFilter, p pagination.Params) ([]Exam, error)
	HasAttempts(ctx context.Context, examID uuid.UUID) (bool, error)

	CreateDefaultConfig(ctx context.Context, examID uuid.UUID) (*ExamConfig, error)
	GetConfig(ctx context.Context, examID uuid.UUID) (*ExamConfig, error)
	SaveConfig(ctx context.Context, cfg *ExamConfig) error

	ListSections(ctx context.Context, examID uuid.UUID) ([]Section, error)
	CreateSection(ctx context.Context, s *Section) error
	UpdateSection(ctx context.Context, s *Section) error
	DeleteSection(ctx context.Context, examID, sectionID uuid.UUID) error
	// ShiftSections adds delta to the position of every section at or after from.
	ShiftSections(ctx context.Context, examID uuid.UUID, from, delta int) error
	SetSectionOrder(ctx context.Context, examID uuid.UUID, ordered []uuid.UUID) error
	ReplacePrerequisites(ctx context.Context, sectionID uuid.UUID, prereqs []Prerequisite) error
	SectionStats(ctx context.Context, examID uuid.UUID) (map[uuid.UUID]SectionStats, error)
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

// ---------------------------------------------------------------------------
// Exams
// ---------------------------------------------------------------------------

var examColumns = strings.Join(database.Columns(&Exam{}), ", ")

func (r *postgresRepository) CreateExam(ctx context.Context, exam *Exam) error {
	if exam.ID == uuid.Nil {
		exam.ID = uuid.New()
	}
	query := `
		INSERT INTO exams (id, org_id, created_by_user_id, title, code, description, instructions, status, passing_marks)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING total_marks, created_at, updated_at
	`
	err := database.Conn(ctx, r.db).QueryRow(ctx, query,
		exam.ID, exam.OrgID, exam.CreatedByUserID, exam.Title, exam.Code, exam.Description,
		exam.Instructions, exam.Status, exam.PassingMarks,
	).Scan(&exam.TotalMarks, &exam.CreatedAt, &exam.UpdatedAt)
	if err != nil {
		if database.IsUniqueViolation(err, "exams_unique_org_code") {
			return ErrExamCodeTaken
		}
		return fmt.Errorf("insert exam: %w", err)
	}
	return nil
}

func (r *postgresRepository) GetExam(ctx context.Context, orgID, examID uuid.UUID) (*Exam, error) {
	return r.getExam(ctx, orgID, examID, false)
}

func (r *postgresRepository) LockExam(ctx context.Context, orgID, examID uuid.UUID) (*Exam, error) {
	return r.getExam(ctx, orgID, examID, true)
}

func (r *postgresRepository) getExam(ctx context.Context, orgID, examID uuid.UUID, lock bool) (*Exam, error) {
	query := `SELECT ` + examColumns + ` FROM exams WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL`
	if lock {
		query += ` FOR UPDATE`
	}
	var exam Exam
	if err := database.Conn(ctx, r.db).QueryRow(ctx, query, examID, orgID).Scan(database.ScanTargets(&exam)...); err != nil {
		if database.IsNoRows(err) {
			return nil, ErrExamNotFound
		}
		return nil, fmt.Errorf("select exam: %w", err)
	}
	return &exam, nil
}

func (r *postgresRepository) UpdateExam(ctx context.Context, exam *Exam) error {
	query := `
		UPDATE exams
		SET title = $2, code = $3, description = $4, instructions = $5, passing_marks = $6,
		    status = $7, total_marks = $8, published_at = $9, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`
	err := database.Conn(ctx, r.db).QueryRow(ctx, query,
		exam.ID, exam.Title, exam.Code, exam.Description, exam.Instructions, exam.PassingMarks,
		exam.Status, exam.TotalMarks, exam.PublishedAt,
	).Scan(&exam.UpdatedAt)
	if err != nil {
		if database.IsNoRows(err) {
			return ErrExamNotFound
		}
		if database.IsUniqueViolation(err, "exams_unique_org_code") {
			return ErrExamCodeTaken
		}
		return fmt.Errorf("update exam: %w", err)
	}
	return nil
}

func (r *postgresRepository) SoftDeleteExam(ctx context.Context, examID uuid.UUID) error {
	tag, err := database.Conn(ctx, r.db).Exec(ctx,
		`UPDATE exams SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`, examID)
	if err != nil {
		return fmt.Errorf("soft delete exam: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrExamNotFound
	}
	return nil
}

func (r *postgresRepository) ListExams(ctx context.Context, orgID uuid.UUID, f ExamFilter, p pagination.Params) ([]Exam, error) {
	var (
		where = []string{"org_id = $1", "deleted_at IS NULL"}
		args  = []any{orgID}
	)
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if f.Status != nil {
		where = append(where, "status = "+arg(*f.Status))
	}
	if f.Search != "" {
		pattern := "%" + escapeLike(f.Search) + "%"
		ph := arg(pattern)
		where = append(where, fmt.Sprintf("(title ILIKE %s OR code ILIKE %s)", ph, ph))
	}
	if p.After != nil {
		where = append(where, fmt.Sprintf("(created_at, id) < (%s, %s)", arg(p.After.Time), arg(p.After.ID)))
	}

	query := `SELECT ` + examColumns + ` FROM exams WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY created_at DESC, id DESC LIMIT ` + arg(p.Limit+1)

	rows, err := database.Conn(ctx, r.db).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list exams: %w", err)
	}
	defer rows.Close()

	var out []Exam
	for rows.Next() {
		var e Exam
		if err := rows.Scan(database.ScanTargets(&e)...); err != nil {
			return nil, fmt.Errorf("scan exam: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *postgresRepository) HasAttempts(ctx context.Context, examID uuid.UUID) (bool, error) {
	var exists bool
	err := database.Conn(ctx, r.db).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM exam_attempts WHERE exam_id = $1)`, examID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check exam attempts: %w", err)
	}
	return exists, nil
}

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

var configColumns = strings.Join(database.Columns(&ExamConfig{}), ", ")

// CreateDefaultConfig inserts a configuration row populated entirely from
// the column defaults, so the schema stays the single source of defaults.
func (r *postgresRepository) CreateDefaultConfig(ctx context.Context, examID uuid.UUID) (*ExamConfig, error) {
	query := `INSERT INTO exam_configurations (exam_id) VALUES ($1) RETURNING ` + configColumns
	var cfg ExamConfig
	if err := database.Conn(ctx, r.db).QueryRow(ctx, query, examID).Scan(database.ScanTargets(&cfg)...); err != nil {
		return nil, fmt.Errorf("insert default exam config: %w", err)
	}
	return &cfg, nil
}

func (r *postgresRepository) GetConfig(ctx context.Context, examID uuid.UUID) (*ExamConfig, error) {
	query := `SELECT ` + configColumns + ` FROM exam_configurations WHERE exam_id = $1`
	var cfg ExamConfig
	if err := database.Conn(ctx, r.db).QueryRow(ctx, query, examID).Scan(database.ScanTargets(&cfg)...); err != nil {
		if database.IsNoRows(err) {
			return nil, ErrExamNotFound
		}
		return nil, fmt.Errorf("select exam config: %w", err)
	}
	return &cfg, nil
}

func (r *postgresRepository) SaveConfig(ctx context.Context, cfg *ExamConfig) error {
	if cfg.CustomSettings == nil {
		cfg.CustomSettings = map[string]any{}
	}
	if cfg.Experience.TimerWarningThresholdsMinutes == nil {
		cfg.Experience.TimerWarningThresholdsMinutes = []int32{}
	}
	exclude := []string{"exam_id", "created_at", "updated_at"}
	cols := database.Columns(cfg, exclude...)
	vals := database.Values(cfg, exclude...)

	query := `UPDATE exam_configurations SET ` + database.SetClause(cols, 2) +
		`, updated_at = now() WHERE exam_id = $1 RETURNING updated_at`
	args := append([]any{cfg.ExamID}, vals...)
	if err := database.Conn(ctx, r.db).QueryRow(ctx, query, args...).Scan(&cfg.UpdatedAt); err != nil {
		if database.IsNoRows(err) {
			return ErrExamNotFound
		}
		return fmt.Errorf("update exam config: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Sections
// ---------------------------------------------------------------------------

var sectionColumns = strings.Join(database.Columns(&Section{}), ", ")

func (r *postgresRepository) ListSections(ctx context.Context, examID uuid.UUID) ([]Section, error) {
	conn := database.Conn(ctx, r.db)

	rows, err := conn.Query(ctx,
		`SELECT `+sectionColumns+` FROM exam_sections WHERE exam_id = $1 ORDER BY position`, examID)
	if err != nil {
		return nil, fmt.Errorf("list sections: %w", err)
	}
	sections, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Section, error) {
		var s Section
		err := row.Scan(database.ScanTargets(&s)...)
		s.Prerequisites = []Prerequisite{}
		return s, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan sections: %w", err)
	}

	index := make(map[uuid.UUID]int, len(sections))
	for i, s := range sections {
		index[s.ID] = i
	}

	prereqRows, err := conn.Query(ctx, `
		SELECT p.section_id, p.prerequisite_section_id, p.min_score_percent
		FROM exam_section_prerequisites p
		JOIN exam_sections s ON s.id = p.section_id
		WHERE s.exam_id = $1
		ORDER BY p.created_at, p.prerequisite_section_id
	`, examID)
	if err != nil {
		return nil, fmt.Errorf("list prerequisites: %w", err)
	}
	defer prereqRows.Close()
	for prereqRows.Next() {
		var sectionID uuid.UUID
		var p Prerequisite
		if err := prereqRows.Scan(&sectionID, &p.SectionID, &p.MinScorePercent); err != nil {
			return nil, fmt.Errorf("scan prerequisite: %w", err)
		}
		if i, ok := index[sectionID]; ok {
			sections[i].Prerequisites = append(sections[i].Prerequisites, p)
		}
	}
	return sections, prereqRows.Err()
}

func (r *postgresRepository) CreateSection(ctx context.Context, s *Section) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	exclude := []string{"created_at", "updated_at"}
	cols := database.Columns(s, exclude...)
	query := `INSERT INTO exam_sections (` + strings.Join(cols, ", ") + `) VALUES (` +
		database.Placeholders(1, len(cols)) + `) RETURNING created_at, updated_at`
	if err := database.Conn(ctx, r.db).QueryRow(ctx, query, database.Values(s, exclude...)...).Scan(&s.CreatedAt, &s.UpdatedAt); err != nil {
		return fmt.Errorf("insert section: %w", err)
	}
	return r.ReplacePrerequisites(ctx, s.ID, s.Prerequisites)
}

func (r *postgresRepository) UpdateSection(ctx context.Context, s *Section) error {
	// Identity, ownership, type and position are managed elsewhere.
	exclude := []string{"id", "exam_id", "section_type", "position", "created_at", "updated_at"}
	cols := database.Columns(s, exclude...)
	query := `UPDATE exam_sections SET ` + database.SetClause(cols, 3) +
		`, updated_at = now() WHERE id = $1 AND exam_id = $2 RETURNING updated_at`
	args := append([]any{s.ID, s.ExamID}, database.Values(s, exclude...)...)
	if err := database.Conn(ctx, r.db).QueryRow(ctx, query, args...).Scan(&s.UpdatedAt); err != nil {
		if database.IsNoRows(err) {
			return ErrSectionNotFound
		}
		return fmt.Errorf("update section: %w", err)
	}
	return r.ReplacePrerequisites(ctx, s.ID, s.Prerequisites)
}

func (r *postgresRepository) DeleteSection(ctx context.Context, examID, sectionID uuid.UUID) error {
	tag, err := database.Conn(ctx, r.db).Exec(ctx,
		`DELETE FROM exam_sections WHERE id = $1 AND exam_id = $2`, sectionID, examID)
	if err != nil {
		return fmt.Errorf("delete section: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSectionNotFound
	}
	return nil
}

// ShiftSections relies on the unique (exam_id, position) constraint being
// DEFERRABLE: uniqueness is then checked at the end of the statement, so
// shifting a contiguous range never trips over its own rows.
func (r *postgresRepository) ShiftSections(ctx context.Context, examID uuid.UUID, from, delta int) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx,
		`UPDATE exam_sections SET position = position + $3, updated_at = now() WHERE exam_id = $1 AND position >= $2`,
		examID, from, delta)
	if err != nil {
		return fmt.Errorf("shift sections: %w", err)
	}
	return nil
}

func (r *postgresRepository) SetSectionOrder(ctx context.Context, examID uuid.UUID, ordered []uuid.UUID) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx, `
		UPDATE exam_sections s
		SET position = o.ord - 1, updated_at = now()
		FROM unnest($2::uuid[]) WITH ORDINALITY AS o(id, ord)
		WHERE s.id = o.id AND s.exam_id = $1 AND s.position <> o.ord - 1
	`, examID, ordered)
	if err != nil {
		return fmt.Errorf("reorder sections: %w", err)
	}
	return nil
}

func (r *postgresRepository) ReplacePrerequisites(ctx context.Context, sectionID uuid.UUID, prereqs []Prerequisite) error {
	conn := database.Conn(ctx, r.db)
	if _, err := conn.Exec(ctx, `DELETE FROM exam_section_prerequisites WHERE section_id = $1`, sectionID); err != nil {
		return fmt.Errorf("clear prerequisites: %w", err)
	}
	if len(prereqs) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, p := range prereqs {
		batch.Queue(`
			INSERT INTO exam_section_prerequisites (section_id, prerequisite_section_id, min_score_percent)
			VALUES ($1, $2, $3)
		`, sectionID, p.SectionID, p.MinScorePercent)
	}
	if err := conn.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("insert prerequisites: %w", err)
	}
	return nil
}

func (r *postgresRepository) SectionStats(ctx context.Context, examID uuid.UUID) (map[uuid.UUID]SectionStats, error) {
	rows, err := database.Conn(ctx, r.db).Query(ctx, `
		SELECT s.id,
		       count(q.id),
		       coalesce(sum(q.points), 0)::float8,
		       coalesce(min(q.points), 0)::float8,
		       coalesce(max(q.points), 0)::float8,
		       count(q.id) FILTER (WHERE q.negative_points > 0)
		FROM exam_sections s
		LEFT JOIN questions q ON q.section_id = s.id
		WHERE s.exam_id = $1
		GROUP BY s.id
	`, examID)
	if err != nil {
		return nil, fmt.Errorf("section stats: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID]SectionStats)
	for rows.Next() {
		var id uuid.UUID
		var st SectionStats
		if err := rows.Scan(&id, &st.QuestionCount, &st.TotalPoints, &st.MinPoints, &st.MaxPoints, &st.NegativeMarked); err != nil {
			return nil, fmt.Errorf("scan section stats: %w", err)
		}
		out[id] = st
	}
	return out, rows.Err()
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
