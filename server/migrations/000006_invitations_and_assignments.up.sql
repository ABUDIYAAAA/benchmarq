-- Organization invitations: at most one pending invitation per email.
UPDATE organization_invitations i
SET status = 'revoked', revoked_at = now(), updated_at = now()
WHERE i.status = 'pending'
  AND EXISTS (
      SELECT 1 FROM organization_invitations n
      WHERE n.org_id = i.org_id AND n.email = i.email AND n.status = 'pending'
        AND (n.created_at, n.id) > (i.created_at, i.id)
  );

CREATE UNIQUE INDEX organization_invitations_one_pending
    ON organization_invitations(org_id, email)
    WHERE status = 'pending';

-- Exam assignments: a user is assigned to an exam at most once.
DELETE FROM exam_assignments a
USING exam_assignments b
WHERE a.exam_id = b.exam_id AND a.user_id = b.user_id
  AND (a.created_at, a.id) > (b.created_at, b.id);

DROP INDEX idx_exam_assignments_exam_user;

ALTER TABLE exam_assignments
    ADD COLUMN assigned_by_user_id UUID
        REFERENCES users(id)
        ON DELETE SET NULL,
    ADD COLUMN invite_email_sent_at TIMESTAMPTZ,

    ADD CONSTRAINT exam_assignments_unique_exam_user
        UNIQUE (exam_id, user_id);
