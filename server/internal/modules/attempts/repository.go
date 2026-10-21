package attempts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrExamNotFound     = errors.New("exam not found or not assigned")
	ErrAttemptNotFound  = errors.New("attempt not found")
	ErrAttemptRunning   = errors.New("an attempt is already running")
	ErrSessionNotFound  = errors.New("session not found")
	ErrLaunchCodeUnused = errors.New("launch code invalid, used or expired")
)

// candidateExam is an exam as seen by one assigned candidate.
type candidateExam struct {
	ExamID        uuid.UUID
	OrgID         uuid.UUID
	OrgName       string
	Title         string
	Description   *string
	Instructions  *string
	Status        exams.ExamStatus
	PassingMarks  *float64
	AssignmentID  uuid.UUID
	StartOverride *time.Time
	EndOverride   *time.Time
	Multiplier    float64
	AttemptsUsed  int
	LastEndedAt   *time.Time
	InProgressID  *uuid.UUID
}

type violationRecord struct {
	Type     string
	Counted  bool
	Metadata map[string]any
}

type questionGrade struct {
	QuestionID uuid.UUID
	Score      float64
	Correct    *bool
}

type Repository interface {
	// ListCandidateExams returns the published exams assigned to the user,
	// plus any exam the user has a running attempt in.
	ListCandidateExams(ctx context.Context, userID uuid.UUID) ([]candidateExam, error)
	GetCandidateExam(ctx context.Context, userID, examID uuid.UUID) (*candidateExam, error)

	CreateLaunchCode(ctx context.Context, userID, examID uuid.UUID, codeHash string, expiresAt time.Time) error
	// ConsumeLaunchCode marks a valid code used and returns who it was for.
	ConsumeLaunchCode(ctx context.Context, codeHash string) (userID, examID uuid.UUID, err error)

	CreateSession(ctx context.Context, s *Session, tokenHash, clientIP, userAgent string) error
	GetSession(ctx context.Context, tokenHash string) (*Session, error)
	BindSession(ctx context.Context, sessionID, attemptID uuid.UUID) error
	// RevokeSessions revokes the user's sessions for an exam except keep.
	RevokeSessions(ctx context.Context, userID, examID, keep uuid.UUID) error

	LockAttempt(ctx context.Context, attemptID uuid.UUID, skipLocked bool) (*Attempt, error)
	CreateAttempt(ctx context.Context, a *Attempt, states []SectionState, paper []PaperQuestion, userAgent string) error
	SaveAttempt(ctx context.Context, a *Attempt) error
	LoadSectionStates(ctx context.Context, attemptID uuid.UUID) ([]SectionState, error)
	SaveSectionStates(ctx context.Context, attemptID uuid.UUID, states []*SectionState) error
	LoadPaper(ctx context.Context, attemptID uuid.UUID) ([]PaperQuestion, error)
	MarkViewed(ctx context.Context, attemptID, questionID uuid.UUID, at time.Time) error

	LoadResponses(ctx context.Context, attemptID uuid.UUID) ([]Response, error)
	SaveResponse(ctx context.Context, attemptID uuid.UUID, r *Response) error
	SaveGrades(ctx context.Context, attemptID uuid.UUID, grades []questionGrade) error

	AddViolation(ctx context.Context, attemptID uuid.UUID, v violationRecord) error
	CountViolations(ctx context.Context, attemptID uuid.UUID) (map[string]int, error)

	// DueAttempts lists running attempts the server must act on without the
	// candidate: past their latest deadline, silent past the grace period
	// where resuming is not allowed, or in an exam that is no longer live.
	DueAttempts(ctx context.Context, limit int) ([]uuid.UUID, error)

	ExamInOrg(ctx context.Context, orgID, examID uuid.UUID) (bool, error)
	ListAttempts(ctx context.Context, examID uuid.UUID, p pagination.Params) ([]AttemptSummary, error)
	ReleaseDevice(ctx context.Context, examID, attemptID uuid.UUID) error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

// ---------------------------------------------------------------------------
// Candidate exams
// ---------------------------------------------------------------------------

const candidateSelect = `
	SELECT e.id, e.org_id, o.name, e.title, e.description, e.instructions, e.status, e.passing_marks::float8,
	       a.id, a.start_time_override, a.end_time_override, a.time_limit_override_multiplier::float8,
	       (SELECT count(*) FROM exam_attempts t
	         WHERE t.exam_id = e.id AND t.user_id = a.user_id AND t.status <> 'in_progress'),
	       (SELECT max(t.submitted_at) FROM exam_attempts t
	         WHERE t.exam_id = e.id AND t.user_id = a.user_id AND t.status <> 'in_progress'),
	       (SELECT t.id FROM exam_attempts t
	         WHERE t.exam_id = e.id AND t.user_id = a.user_id AND t.status = 'in_progress')
	FROM exam_assignments a
	JOIN exams e ON e.id = a.exam_id AND e.deleted_at IS NULL
	JOIN organizations o ON o.id = e.org_id AND o.deleted_at IS NULL
	JOIN organization_members m ON m.org_id = e.org_id AND m.user_id = a.user_id
	WHERE a.user_id = $1 AND a.is_active
`

func scanCandidate(row interface{ Scan(...any) error }, c *candidateExam) error {
	return row.Scan(&c.ExamID, &c.OrgID, &c.OrgName, &c.Title, &c.Description, &c.Instructions, &c.Status, &c.PassingMarks,
		&c.AssignmentID, &c.StartOverride, &c.EndOverride, &c.Multiplier, &c.AttemptsUsed, &c.LastEndedAt, &c.InProgressID)
}

func (r *postgresRepository) ListCandidateExams(ctx context.Context, userID uuid.UUID) ([]candidateExam, error) {
	rows, err := database.Conn(ctx, r.db).Query(ctx, candidateSelect+`
		AND (e.status = 'published' OR EXISTS (
			SELECT 1 FROM exam_attempts t WHERE t.exam_id = e.id AND t.user_id = a.user_id AND t.status = 'in_progress'))
		ORDER BY e.title, e.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list candidate exams: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (candidateExam, error) {
		var c candidateExam
		return c, scanCandidate(row, &c)
	})
	if err != nil {
		return nil, fmt.Errorf("scan candidate exams: %w", err)
	}
	return out, nil
}

func (r *postgresRepository) GetCandidateExam(ctx context.Context, userID, examID uuid.UUID) (*candidateExam, error) {
	var c candidateExam
	err := scanCandidate(database.Conn(ctx, r.db).QueryRow(ctx, candidateSelect+` AND e.id = $2`, userID, examID), &c)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, ErrExamNotFound
		}
		return nil, fmt.Errorf("select candidate exam: %w", err)
	}
	return &c, nil
}

// ---------------------------------------------------------------------------
// Launch codes and sessions
// ---------------------------------------------------------------------------

func (r *postgresRepository) CreateLaunchCode(ctx context.Context, userID, examID uuid.UUID, codeHash string, expiresAt time.Time) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx, `
		INSERT INTO attempt_launch_codes (code_hash, user_id, exam_id, expires_at) VALUES ($1, $2, $3, $4)
	`, codeHash, userID, examID, expiresAt)
	if err != nil {
		return fmt.Errorf("insert launch code: %w", err)
	}
	return nil
}

func (r *postgresRepository) ConsumeLaunchCode(ctx context.Context, codeHash string) (uuid.UUID, uuid.UUID, error) {
	var userID, examID uuid.UUID
	err := database.Conn(ctx, r.db).QueryRow(ctx, `
		UPDATE attempt_launch_codes SET used_at = now()
		WHERE code_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id, exam_id
	`, codeHash).Scan(&userID, &examID)
	if err != nil {
		if database.IsNoRows(err) {
			return uuid.Nil, uuid.Nil, ErrLaunchCodeUnused
		}
		return uuid.Nil, uuid.Nil, fmt.Errorf("consume launch code: %w", err)
	}
	return userID, examID, nil
}

func (r *postgresRepository) CreateSession(ctx context.Context, s *Session, tokenHash, clientIP, userAgent string) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	_, err := database.Conn(ctx, r.db).Exec(ctx, `
		INSERT INTO attempt_sessions (id, token_hash, user_id, exam_id, attempt_id, device_id, client_ip, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::inet, NULLIF($8, ''), $9)
	`, s.ID, tokenHash, s.UserID, s.ExamID, s.AttemptID, s.DeviceID, clientIP, userAgent, s.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (r *postgresRepository) GetSession(ctx context.Context, tokenHash string) (*Session, error) {
	var s Session
	err := database.Conn(ctx, r.db).QueryRow(ctx, `
		SELECT id, user_id, exam_id, attempt_id, device_id, expires_at
		FROM attempt_sessions
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
	`, tokenHash).Scan(&s.ID, &s.UserID, &s.ExamID, &s.AttemptID, &s.DeviceID, &s.ExpiresAt)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("select session: %w", err)
	}
	return &s, nil
}

func (r *postgresRepository) BindSession(ctx context.Context, sessionID, attemptID uuid.UUID) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx,
		`UPDATE attempt_sessions SET attempt_id = $2 WHERE id = $1`, sessionID, attemptID)
	if err != nil {
		return fmt.Errorf("bind session: %w", err)
	}
	return nil
}

func (r *postgresRepository) RevokeSessions(ctx context.Context, userID, examID, keep uuid.UUID) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx, `
		UPDATE attempt_sessions SET revoked_at = now()
		WHERE user_id = $1 AND exam_id = $2 AND id <> $3 AND revoked_at IS NULL
	`, userID, examID, keep)
	if err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Attempts
// ---------------------------------------------------------------------------

const attemptColumns = `
	id, exam_id, user_id, assignment_id, attempt_number, status, device_id, seed, started_at,
	timer_origin_at, limit_seconds, window_end_at, time_multiplier::float8, current_section_id,
	paused_at, pause_count, paused_seconds, last_heartbeat_at, deadline_at, submitted_at,
	ended_reason, total_score::float8, max_score::float8, passed, host(client_ip)
`

func scanAttempt(row interface{ Scan(...any) error }, a *Attempt) error {
	return row.Scan(&a.ID, &a.ExamID, &a.UserID, &a.AssignmentID, &a.AttemptNumber, &a.Status, &a.DeviceID, &a.Seed, &a.StartedAt,
		&a.TimerOriginAt, &a.LimitSeconds, &a.WindowEndAt, &a.TimeMultiplier, &a.CurrentSectionID,
		&a.PausedAt, &a.PauseCount, &a.PausedSeconds, &a.LastHeartbeatAt, &a.DeadlineAt, &a.SubmittedAt,
		&a.EndedReason, &a.TotalScore, &a.MaxScore, &a.Passed, &a.ClientIP)
}

func (r *postgresRepository) LockAttempt(ctx context.Context, attemptID uuid.UUID, skipLocked bool) (*Attempt, error) {
	query := `SELECT ` + attemptColumns + ` FROM exam_attempts WHERE id = $1 FOR UPDATE`
	if skipLocked {
		query += ` SKIP LOCKED`
	}
	var a Attempt
	if err := scanAttempt(database.Conn(ctx, r.db).QueryRow(ctx, query, attemptID), &a); err != nil {
		if database.IsNoRows(err) {
			return nil, ErrAttemptNotFound
		}
		return nil, fmt.Errorf("lock attempt: %w", err)
	}
	return &a, nil
}

func (r *postgresRepository) CreateAttempt(ctx context.Context, a *Attempt, states []SectionState, paper []PaperQuestion, userAgent string) error {
	conn := database.Conn(ctx, r.db)
	err := conn.QueryRow(ctx, `
		INSERT INTO exam_attempts (
			id, exam_id, user_id, assignment_id, attempt_number, status, device_id, seed, started_at,
			timer_origin_at, limit_seconds, window_end_at, time_multiplier, last_heartbeat_at, deadline_at,
			client_ip, user_agent
		) VALUES (
			$1, $2, $3, $4,
			(SELECT coalesce(max(attempt_number), 0) + 1 FROM exam_attempts WHERE exam_id = $2 AND user_id = $3),
			'in_progress', $5, $6, $7, $8, $9, $10, $11, $12, $13, NULLIF($14, '')::inet, NULLIF($15, '')
		)
		RETURNING attempt_number
	`, a.ID, a.ExamID, a.UserID, a.AssignmentID, a.DeviceID, a.Seed, a.StartedAt,
		a.TimerOriginAt, a.LimitSeconds, a.WindowEndAt, a.TimeMultiplier, a.LastHeartbeatAt, a.DeadlineAt,
		deref(a.ClientIP), userAgent,
	).Scan(&a.AttemptNumber)
	if err != nil {
		if database.IsUniqueViolation(err, "exam_attempts_one_in_progress") ||
			database.IsUniqueViolation(err, "exam_attempts_unique_user_attempt") {
			return ErrAttemptRunning
		}
		return fmt.Errorf("insert attempt: %w", err)
	}

	batch := &pgx.Batch{}
	for _, s := range states {
		batch.Queue(`
			INSERT INTO attempt_section_states (attempt_id, section_id, position, status, time_limit_seconds)
			VALUES ($1, $2, $3, $4, $5)
		`, a.ID, s.SectionID, s.Position, s.Status, s.TimeLimitSeconds)
	}
	for _, q := range paper {
		batch.Queue(`
			INSERT INTO attempt_questions (attempt_id, question_id, section_id, position, option_order, points)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, a.ID, q.QuestionID, q.SectionID, q.Position, q.OptionOrder, q.Points)
	}
	if err := conn.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("insert attempt paper: %w", err)
	}
	return nil
}

func (r *postgresRepository) SaveAttempt(ctx context.Context, a *Attempt) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx, `
		UPDATE exam_attempts SET
			status = $2, device_id = $3, current_section_id = $4, paused_at = $5, pause_count = $6,
			paused_seconds = $7, last_heartbeat_at = $8, deadline_at = $9, submitted_at = $10,
			ended_reason = $11, total_score = $12, max_score = $13, passed = $14,
			client_ip = NULLIF($15, '')::inet, updated_at = now()
		WHERE id = $1
	`, a.ID, a.Status, a.DeviceID, a.CurrentSectionID, a.PausedAt, a.PauseCount,
		a.PausedSeconds, a.LastHeartbeatAt, a.DeadlineAt, a.SubmittedAt,
		a.EndedReason, a.TotalScore, a.MaxScore, a.Passed, deref(a.ClientIP))
	if err != nil {
		return fmt.Errorf("update attempt: %w", err)
	}
	return nil
}

func (r *postgresRepository) LoadSectionStates(ctx context.Context, attemptID uuid.UUID) ([]SectionState, error) {
	rows, err := database.Conn(ctx, r.db).Query(ctx, `
		SELECT section_id, position, status, time_limit_seconds, time_spent_seconds, entered_at,
		       unlocked_at, completed_at, cursor, score_awarded::float8, max_score::float8
		FROM attempt_section_states WHERE attempt_id = $1 ORDER BY position
	`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("load section states: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (SectionState, error) {
		var s SectionState
		err := row.Scan(&s.SectionID, &s.Position, &s.Status, &s.TimeLimitSeconds, &s.TimeSpentSeconds, &s.EnteredAt,
			&s.UnlockedAt, &s.CompletedAt, &s.Cursor, &s.ScoreAwarded, &s.MaxScore)
		return s, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan section states: %w", err)
	}
	return out, nil
}

func (r *postgresRepository) SaveSectionStates(ctx context.Context, attemptID uuid.UUID, states []*SectionState) error {
	batch := &pgx.Batch{}
	for _, s := range states {
		batch.Queue(`
			UPDATE attempt_section_states SET
				status = $3, time_spent_seconds = $4, entered_at = $5, unlocked_at = $6,
				completed_at = $7, cursor = $8, score_awarded = $9, max_score = $10
			WHERE attempt_id = $1 AND section_id = $2
		`, attemptID, s.SectionID, s.Status, s.TimeSpentSeconds, s.EnteredAt, s.UnlockedAt,
			s.CompletedAt, s.Cursor, s.ScoreAwarded, s.MaxScore)
	}
	if err := database.Conn(ctx, r.db).SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("update section states: %w", err)
	}
	return nil
}

func (r *postgresRepository) LoadPaper(ctx context.Context, attemptID uuid.UUID) ([]PaperQuestion, error) {
	rows, err := database.Conn(ctx, r.db).Query(ctx, `
		SELECT question_id, section_id, position, option_order, points::float8, first_viewed_at
		FROM attempt_questions WHERE attempt_id = $1 ORDER BY section_id, position
	`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("load paper: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (PaperQuestion, error) {
		var q PaperQuestion
		err := row.Scan(&q.QuestionID, &q.SectionID, &q.Position, &q.OptionOrder, &q.Points, &q.FirstViewedAt)
		return q, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan paper: %w", err)
	}
	return out, nil
}

func (r *postgresRepository) MarkViewed(ctx context.Context, attemptID, questionID uuid.UUID, at time.Time) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx, `
		UPDATE attempt_questions SET first_viewed_at = $3
		WHERE attempt_id = $1 AND question_id = $2 AND first_viewed_at IS NULL
	`, attemptID, questionID, at)
	if err != nil {
		return fmt.Errorf("mark question viewed: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Responses
// ---------------------------------------------------------------------------

func (r *postgresRepository) LoadResponses(ctx context.Context, attemptID uuid.UUID) ([]Response, error) {
	rows, err := database.Conn(ctx, r.db).Query(ctx, `
		SELECT question_id, selected_option_id, selected_option_ids, text_response, boolean_response,
		       is_flagged_for_review, updated_at
		FROM attempt_responses WHERE attempt_id = $1
	`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("load responses: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Response, error) {
		var x Response
		err := row.Scan(&x.QuestionID, &x.SelectedOptionID, &x.SelectedOptionIDs, &x.TextResponse, &x.BooleanResponse,
			&x.Flagged, &x.UpdatedAt)
		return x, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan responses: %w", err)
	}
	return out, nil
}

func (r *postgresRepository) SaveResponse(ctx context.Context, attemptID uuid.UUID, x *Response) error {
	var ids []uuid.UUID
	if len(x.SelectedOptionIDs) > 0 {
		ids = x.SelectedOptionIDs
	}
	err := database.Conn(ctx, r.db).QueryRow(ctx, `
		INSERT INTO attempt_responses (attempt_id, question_id, selected_option_id, selected_option_ids,
		                               text_response, boolean_response, is_flagged_for_review)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (attempt_id, question_id) DO UPDATE SET
			selected_option_id = EXCLUDED.selected_option_id,
			selected_option_ids = EXCLUDED.selected_option_ids,
			text_response = EXCLUDED.text_response,
			boolean_response = EXCLUDED.boolean_response,
			is_flagged_for_review = EXCLUDED.is_flagged_for_review,
			updated_at = now()
		RETURNING updated_at
	`, attemptID, x.QuestionID, x.SelectedOptionID, ids, x.TextResponse, x.BooleanResponse, x.Flagged).Scan(&x.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save response: %w", err)
	}
	return nil
}

func (r *postgresRepository) SaveGrades(ctx context.Context, attemptID uuid.UUID, grades []questionGrade) error {
	batch := &pgx.Batch{}
	for _, g := range grades {
		batch.Queue(`
			UPDATE attempt_responses SET is_correct = $3, score_awarded = $4, evaluated_at = now()
			WHERE attempt_id = $1 AND question_id = $2
		`, attemptID, g.QuestionID, g.Correct, g.Score)
	}
	if batch.Len() == 0 {
		return nil
	}
	if err := database.Conn(ctx, r.db).SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("save grades: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Proctoring
// ---------------------------------------------------------------------------

func (r *postgresRepository) AddViolation(ctx context.Context, attemptID uuid.UUID, v violationRecord) error {
	meta := v.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	_, err := database.Conn(ctx, r.db).Exec(ctx, `
		INSERT INTO proctoring_violation_logs (attempt_id, violation_type, counted, metadata) VALUES ($1, $2, $3, $4)
	`, attemptID, v.Type, v.Counted, meta)
	if err != nil {
		return fmt.Errorf("insert violation: %w", err)
	}
	return nil
}

func (r *postgresRepository) CountViolations(ctx context.Context, attemptID uuid.UUID) (map[string]int, error) {
	rows, err := database.Conn(ctx, r.db).Query(ctx, `
		SELECT violation_type::text, count(*) FROM proctoring_violation_logs
		WHERE attempt_id = $1 AND counted GROUP BY violation_type
	`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("count violations: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var t string
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, fmt.Errorf("scan violation count: %w", err)
		}
		out[t] = n
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Background and staff
// ---------------------------------------------------------------------------

func (r *postgresRepository) DueAttempts(ctx context.Context, limit int) ([]uuid.UUID, error) {
	rows, err := database.Conn(ctx, r.db).Query(ctx, `
		SELECT t.id
		FROM exam_attempts t
		JOIN exam_configurations c ON c.exam_id = t.exam_id
		JOIN exams e ON e.id = t.exam_id
		WHERE t.status = 'in_progress'
		  AND (
		      t.deadline_at <= now()
		      OR e.status <> 'published' OR e.deleted_at IS NOT NULL
		      OR (NOT c.allow_resume_on_disconnect AND t.paused_at IS NULL
		          AND t.last_heartbeat_at < now() - make_interval(secs => c.reconnection_grace_seconds))
		  )
		ORDER BY t.deadline_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list due attempts: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, fmt.Errorf("scan due attempts: %w", err)
	}
	return ids, nil
}

func (r *postgresRepository) ExamInOrg(ctx context.Context, orgID, examID uuid.UUID) (bool, error) {
	var ok bool
	err := database.Conn(ctx, r.db).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM exams WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL)`, examID, orgID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check exam: %w", err)
	}
	return ok, nil
}

func (r *postgresRepository) ListAttempts(ctx context.Context, examID uuid.UUID, p pagination.Params) ([]AttemptSummary, error) {
	args := []any{examID, p.Limit + 1}
	cursorClause := ""
	if p.After != nil {
		cursorClause = `AND (t.started_at, t.id) < ($3, $4)`
		args = append(args, p.After.Time, p.After.ID)
	}
	rows, err := database.Conn(ctx, r.db).Query(ctx, `
		SELECT t.id, t.user_id, u.email, u.first_name, u.last_name, t.attempt_number, t.status, t.started_at,
		       t.submitted_at, t.ended_reason, t.device_id IS NOT NULL, t.last_heartbeat_at, t.deadline_at,
		       t.total_score::float8, t.max_score::float8
		FROM exam_attempts t
		JOIN users u ON u.id = t.user_id
		WHERE t.exam_id = $1 `+cursorClause+`
		ORDER BY t.started_at DESC, t.id DESC
		LIMIT $2
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("list attempts: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AttemptSummary, error) {
		var s AttemptSummary
		err := row.Scan(&s.ID, &s.UserID, &s.Email, &s.FirstName, &s.LastName, &s.AttemptNumber, &s.Status, &s.StartedAt,
			&s.SubmittedAt, &s.EndedReason, &s.DeviceBound, &s.LastHeartbeatAt, &s.DeadlineAt, &s.TotalScore, &s.MaxScore)
		return s, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan attempts: %w", err)
	}
	return out, nil
}

func (r *postgresRepository) ReleaseDevice(ctx context.Context, examID, attemptID uuid.UUID) error {
	conn := database.Conn(ctx, r.db)
	tag, err := conn.Exec(ctx, `
		UPDATE exam_attempts SET device_id = NULL, updated_at = now()
		WHERE id = $1 AND exam_id = $2 AND status = 'in_progress'
	`, attemptID, examID)
	if err != nil {
		return fmt.Errorf("release device: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAttemptNotFound
	}
	_, err = conn.Exec(ctx, `UPDATE attempt_sessions SET revoked_at = now() WHERE attempt_id = $1 AND revoked_at IS NULL`, attemptID)
	if err != nil {
		return fmt.Errorf("revoke attempt sessions: %w", err)
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
