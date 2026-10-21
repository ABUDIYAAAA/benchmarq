package assignments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrExamNotFound       = errors.New("exam not found")
	ErrAssignmentNotFound = errors.New("assignment not found")
)

type Repository interface {
	// LockExam loads the org's exam with a share lock, so its status cannot
	// change (e.g. be unpublished) while assignments are written.
	LockExam(ctx context.Context, orgID, examID uuid.UUID) (*examRef, error)
	GetExam(ctx context.Context, orgID, examID uuid.UUID) (*examRef, error)
	// NonMembers returns the IDs in userIDs that are not members of the org.
	NonMembers(ctx context.Context, orgID uuid.UUID, userIDs []uuid.UUID) ([]uuid.UUID, error)
	// Assign creates missing assignments and returns how many were created.
	Assign(ctx context.Context, examID, assignedBy uuid.UUID, userIDs []uuid.UUID) (int, error)
	ListForUsers(ctx context.Context, examID uuid.UUID, userIDs []uuid.UUID) ([]Assignment, error)
	List(ctx context.Context, examID uuid.UUID, p pagination.Params) ([]Assignment, error)
	Delete(ctx context.Context, examID, assignmentID uuid.UUID) error
	MarkInviteSent(ctx context.Context, assignmentIDs []uuid.UUID, at time.Time) error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) LockExam(ctx context.Context, orgID, examID uuid.UUID) (*examRef, error) {
	return r.exam(ctx, orgID, examID, true)
}

func (r *postgresRepository) GetExam(ctx context.Context, orgID, examID uuid.UUID) (*examRef, error) {
	return r.exam(ctx, orgID, examID, false)
}

func (r *postgresRepository) exam(ctx context.Context, orgID, examID uuid.UUID, lock bool) (*examRef, error) {
	query := `
		SELECT e.title, e.status, c.schedule_type, c.start_time, c.total_duration_minutes
		FROM exams e
		JOIN exam_configurations c ON c.exam_id = e.id
		WHERE e.id = $1 AND e.org_id = $2 AND e.deleted_at IS NULL
	`
	if lock {
		query += ` FOR SHARE OF e`
	}
	var ref examRef
	err := database.Conn(ctx, r.db).QueryRow(ctx, query, examID, orgID).Scan(
		&ref.Title, &ref.Status, &ref.ScheduleType, &ref.StartTime, &ref.TotalDurationMinutes)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, ErrExamNotFound
		}
		return nil, fmt.Errorf("select exam: %w", err)
	}
	return &ref, nil
}

func (r *postgresRepository) NonMembers(ctx context.Context, orgID uuid.UUID, userIDs []uuid.UUID) ([]uuid.UUID, error) {
	rows, err := database.Conn(ctx, r.db).Query(ctx, `
		SELECT req.user_id FROM unnest($2::uuid[]) AS req(user_id)
		WHERE NOT EXISTS (
			SELECT 1 FROM organization_members m
			JOIN users u ON u.id = m.user_id AND u.deleted_at IS NULL
			WHERE m.org_id = $1 AND m.user_id = req.user_id
		)
	`, orgID, userIDs)
	if err != nil {
		return nil, fmt.Errorf("check members: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, fmt.Errorf("scan non-members: %w", err)
	}
	return ids, nil
}

func (r *postgresRepository) Assign(ctx context.Context, examID, assignedBy uuid.UUID, userIDs []uuid.UUID) (int, error) {
	tag, err := database.Conn(ctx, r.db).Exec(ctx, `
		INSERT INTO exam_assignments (exam_id, user_id, assigned_by_user_id)
		SELECT $1, req.user_id, $2 FROM unnest($3::uuid[]) AS req(user_id)
		ON CONFLICT (exam_id, user_id) DO NOTHING
	`, examID, assignedBy, userIDs)
	if err != nil {
		return 0, fmt.Errorf("insert assignments: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

const assignmentSelect = `
	SELECT a.id, a.exam_id, a.user_id, u.email, u.first_name, u.last_name,
	       a.assigned_by_user_id, a.invite_email_sent_at, a.created_at
	FROM exam_assignments a
	JOIN users u ON u.id = a.user_id AND u.deleted_at IS NULL
`

func scanAssignment(row pgx.CollectableRow) (Assignment, error) {
	var a Assignment
	err := row.Scan(&a.ID, &a.ExamID, &a.UserID, &a.Email, &a.FirstName, &a.LastName,
		&a.AssignedByUserID, &a.InviteEmailSentAt, &a.CreatedAt)
	return a, err
}

func (r *postgresRepository) ListForUsers(ctx context.Context, examID uuid.UUID, userIDs []uuid.UUID) ([]Assignment, error) {
	rows, err := database.Conn(ctx, r.db).Query(ctx,
		assignmentSelect+` WHERE a.exam_id = $1 AND a.user_id = ANY($2) ORDER BY a.created_at, a.id`, examID, userIDs)
	if err != nil {
		return nil, fmt.Errorf("list assignments for users: %w", err)
	}
	out, err := pgx.CollectRows(rows, scanAssignment)
	if err != nil {
		return nil, fmt.Errorf("scan assignments: %w", err)
	}
	return out, nil
}

func (r *postgresRepository) List(ctx context.Context, examID uuid.UUID, p pagination.Params) ([]Assignment, error) {
	args := []any{examID, p.Limit + 1}
	cursorClause := ""
	if p.After != nil {
		cursorClause = `AND (a.created_at, a.id) > ($3, $4)`
		args = append(args, p.After.Time, p.After.ID)
	}
	rows, err := database.Conn(ctx, r.db).Query(ctx, assignmentSelect+` WHERE a.exam_id = $1 `+cursorClause+`
		ORDER BY a.created_at ASC, a.id ASC
		LIMIT $2`, args...)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	out, err := pgx.CollectRows(rows, scanAssignment)
	if err != nil {
		return nil, fmt.Errorf("scan assignments: %w", err)
	}
	return out, nil
}

func (r *postgresRepository) Delete(ctx context.Context, examID, assignmentID uuid.UUID) error {
	tag, err := database.Conn(ctx, r.db).Exec(ctx,
		`DELETE FROM exam_assignments WHERE id = $1 AND exam_id = $2`, assignmentID, examID)
	if err != nil {
		return fmt.Errorf("delete assignment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAssignmentNotFound
	}
	return nil
}

func (r *postgresRepository) MarkInviteSent(ctx context.Context, assignmentIDs []uuid.UUID, at time.Time) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx, `
		UPDATE exam_assignments SET invite_email_sent_at = $2, updated_at = now()
		WHERE id = ANY($1)
	`, assignmentIDs, at)
	if err != nil {
		return fmt.Errorf("mark invitations sent: %w", err)
	}
	return nil
}
