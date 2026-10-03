CREATE TYPE attempt_status AS ENUM (
    'in_progress',
    'submitted',
    'auto_submitted',
    'disqualified',
    'abandoned',
    'evaluated'
);

CREATE TYPE section_attempt_status AS ENUM (
    'locked',
    'in_progress',
    'completed',
    'skipped'
);

CREATE TYPE proctoring_violation_type AS ENUM (
    'fullscreen_exit',
    'tab_switch',
    'multiple_faces',
    'no_face',
    'audio_detected',
    'clipboard_action',
    'dev_tools_opened',
    'ip_changed'
);

-- Exam Assignments (Specific candidates or open enrollment)
CREATE TABLE exam_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    exam_id UUID NOT NULL
        REFERENCES exams(id)
        ON DELETE CASCADE,

    user_id UUID
        REFERENCES users(id)
        ON DELETE CASCADE,

    invited_email CITEXT,
    access_code VARCHAR(50),

    time_limit_override_multiplier NUMERIC(3, 2) NOT NULL DEFAULT 1.00,
    start_time_override TIMESTAMPTZ,
    end_time_override TIMESTAMPTZ,

    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_exam_assignments_exam_user ON exam_assignments(exam_id, user_id);
CREATE INDEX idx_exam_assignments_access_code ON exam_assignments(access_code);

-- Exam Attempt Session
CREATE TABLE exam_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    exam_id UUID NOT NULL
        REFERENCES exams(id)
        ON DELETE CASCADE,

    user_id UUID NOT NULL
        REFERENCES users(id)
        ON DELETE CASCADE,

    attempt_number INTEGER NOT NULL DEFAULT 1,
    status attempt_status NOT NULL DEFAULT 'in_progress',

    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at TIMESTAMPTZ,

    total_score NUMERIC(8, 2),
    passed BOOLEAN,
    feedback TEXT,

    client_ip INET,
    user_agent TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT exam_attempts_attempt_positive
        CHECK (attempt_number >= 1),

    CONSTRAINT exam_attempts_unique_user_attempt
        UNIQUE (exam_id, user_id, attempt_number)
);

CREATE INDEX idx_exam_attempts_exam_user ON exam_attempts(exam_id, user_id);
CREATE INDEX idx_exam_attempts_status ON exam_attempts(status);

-- Section States per Attempt (For Linear Section Gating and Timing)
CREATE TABLE attempt_section_states (
    attempt_id UUID NOT NULL
        REFERENCES exam_attempts(id)
        ON DELETE CASCADE,

    section_id UUID NOT NULL
        REFERENCES exam_sections(id)
        ON DELETE CASCADE,

    status section_attempt_status NOT NULL DEFAULT 'locked',

    unlocked_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    time_spent_seconds INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY (attempt_id, section_id),

    CONSTRAINT attempt_section_time_spent_positive
        CHECK (time_spent_seconds >= 0)
);

-- Objective & Subjective Question Responses
CREATE TABLE attempt_responses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    attempt_id UUID NOT NULL
        REFERENCES exam_attempts(id)
        ON DELETE CASCADE,

    question_id UUID NOT NULL
        REFERENCES questions(id)
        ON DELETE CASCADE,

    selected_option_id UUID
        REFERENCES question_options(id)
        ON DELETE SET NULL,

    selected_option_ids UUID[],
    text_response TEXT,
    boolean_response BOOLEAN,

    is_flagged_for_review BOOLEAN NOT NULL DEFAULT FALSE,

    is_correct BOOLEAN,
    score_awarded NUMERIC(6, 2),
    evaluator_feedback TEXT,

    evaluated_by_user_id UUID
        REFERENCES users(id)
        ON DELETE SET NULL,

    evaluated_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT attempt_responses_unique_attempt_question
        UNIQUE (attempt_id, question_id)
);

CREATE INDEX idx_attempt_responses_attempt_id ON attempt_responses(attempt_id);

-- Coding Submissions & Test Runs
CREATE TABLE attempt_coding_submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    attempt_id UUID NOT NULL
        REFERENCES exam_attempts(id)
        ON DELETE CASCADE,

    question_id UUID NOT NULL
        REFERENCES coding_questions(question_id)
        ON DELETE CASCADE,

    language_id UUID NOT NULL
        REFERENCES coding_languages(id)
        ON DELETE RESTRICT,

    submitted_code TEXT NOT NULL,
    execution_status VARCHAR(50) NOT NULL DEFAULT 'pending',

    total_test_cases INTEGER NOT NULL DEFAULT 0,
    passed_test_cases INTEGER NOT NULL DEFAULT 0,
    score_awarded NUMERIC(6, 2) NOT NULL DEFAULT 0.00,

    execution_time_ms INTEGER,
    memory_used_kb INTEGER,
    compiler_output TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_attempt_coding_submissions_attempt_q ON attempt_coding_submissions(attempt_id, question_id);

-- Proctoring Violation Audit Logs
CREATE TABLE proctoring_violation_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    attempt_id UUID NOT NULL
        REFERENCES exam_attempts(id)
        ON DELETE CASCADE,

    violation_type proctoring_violation_type NOT NULL,
    evidence_snapshot_url TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,

    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    is_reviewed BOOLEAN NOT NULL DEFAULT FALSE,
    reviewed_by_user_id UUID
        REFERENCES users(id)
        ON DELETE SET NULL,

    reviewer_notes TEXT
);

CREATE INDEX idx_proctoring_violation_attempt_id ON proctoring_violation_logs(attempt_id);
