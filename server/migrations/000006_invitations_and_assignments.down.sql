ALTER TABLE exam_assignments
    DROP CONSTRAINT exam_assignments_unique_exam_user,
    DROP COLUMN invite_email_sent_at,
    DROP COLUMN assigned_by_user_id;

CREATE INDEX idx_exam_assignments_exam_user ON exam_assignments(exam_id, user_id);

DROP INDEX organization_invitations_one_pending;
