DROP INDEX IF EXISTS idx_proctoring_violation_attempt_type;
ALTER TABLE proctoring_violation_logs DROP COLUMN counted;

DROP TABLE attempt_sessions;
DROP TABLE attempt_launch_codes;
DROP TABLE attempt_questions;

ALTER TABLE attempt_section_states
    DROP CONSTRAINT attempt_section_states_unique_position,
    DROP COLUMN max_score,
    DROP COLUMN score_awarded,
    DROP COLUMN cursor,
    DROP COLUMN entered_at,
    DROP COLUMN time_limit_seconds,
    DROP COLUMN position;

DROP INDEX idx_exam_attempts_running_deadline;
DROP INDEX exam_attempts_one_in_progress;

ALTER TABLE exam_attempts
    DROP CONSTRAINT exam_attempts_paused_seconds_non_negative,
    DROP CONSTRAINT exam_attempts_pause_count_non_negative,
    DROP COLUMN max_score,
    DROP COLUMN ended_reason,
    DROP COLUMN deadline_at,
    DROP COLUMN last_heartbeat_at,
    DROP COLUMN paused_seconds,
    DROP COLUMN pause_count,
    DROP COLUMN paused_at,
    DROP COLUMN current_section_id,
    DROP COLUMN time_multiplier,
    DROP COLUMN window_end_at,
    DROP COLUMN limit_seconds,
    DROP COLUMN timer_origin_at,
    DROP COLUMN seed,
    DROP COLUMN device_id,
    DROP COLUMN assignment_id;
