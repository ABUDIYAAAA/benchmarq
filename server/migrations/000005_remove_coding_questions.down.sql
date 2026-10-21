-- Restores the coding schema. Coding sections and questions deleted by the
-- up migration are not recovered.

ALTER TABLE essay_questions
    ADD COLUMN allow_file_attachments BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE exam_configurations
    ADD COLUMN allowed_language_ids UUID[],
    ADD COLUMN code_run_limit_per_question INTEGER NOT NULL DEFAULT 30,
    ADD COLUMN editor_autocomplete_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN allow_custom_test_input BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN show_failed_testcase_diff BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN default_timeout_ms INTEGER NOT NULL DEFAULT 2000,
    ADD COLUMN default_memory_limit_mb INTEGER NOT NULL DEFAULT 256;

ALTER TABLE questions
    DROP CONSTRAINT question_type_matches_section_family,
    DROP CONSTRAINT questions_section_fk;

ALTER TABLE exam_sections
    DROP CONSTRAINT exam_sections_unique_id_type;

ALTER TYPE section_type RENAME TO section_type_old;
CREATE TYPE section_type AS ENUM (
    'objective',
    'coding',
    'subjective'
);
ALTER TABLE exam_sections ALTER COLUMN section_type TYPE section_type USING section_type::text::section_type;
ALTER TABLE questions ALTER COLUMN section_type TYPE section_type USING section_type::text::section_type;
DROP TYPE section_type_old;

ALTER TYPE question_type RENAME TO question_type_old;
CREATE TYPE question_type AS ENUM (
    'multiple_choice',
    'true_false',
    'multi_select',
    'blank_fill',
    'coding',
    'short_answer',
    'essay'
);
ALTER TABLE questions ALTER COLUMN type TYPE question_type USING type::text::question_type;
DROP TYPE question_type_old;

ALTER TABLE exam_sections
    ADD CONSTRAINT exam_sections_unique_id_type
        UNIQUE (id, section_type);

ALTER TABLE questions
    ADD CONSTRAINT questions_section_fk
        FOREIGN KEY (section_id, section_type)
        REFERENCES exam_sections(id, section_type)
        ON DELETE CASCADE,

    ADD CONSTRAINT question_type_matches_section_family
        CHECK (
            (section_type = 'objective' AND type IN ('multiple_choice', 'true_false', 'multi_select', 'blank_fill')) OR
            (section_type = 'coding' AND type = 'coding') OR
            (section_type = 'subjective' AND type IN ('short_answer', 'essay'))
        );

CREATE TABLE coding_languages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name VARCHAR(100) NOT NULL UNIQUE,
    slug VARCHAR(50) NOT NULL UNIQUE,
    judge0_language_id INTEGER UNIQUE,
    version VARCHAR(50),

    compile_command TEXT,
    run_command TEXT,
    default_starter_code TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE coding_questions (
    question_id UUID PRIMARY KEY
        REFERENCES questions(id)
        ON DELETE CASCADE,

    default_language_id UUID
        REFERENCES coding_languages(id)
        ON DELETE SET NULL,

    starter_code TEXT,
    solution_code TEXT,
    driver_code TEXT,

    time_limit_ms INTEGER NOT NULL DEFAULT 2000,
    memory_limit_mb INTEGER NOT NULL DEFAULT 128,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT coding_time_limit_positive
        CHECK (time_limit_ms > 0),

    CONSTRAINT coding_memory_limit_positive
        CHECK (memory_limit_mb > 0)
);

CREATE TABLE coding_test_cases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    question_id UUID NOT NULL
        REFERENCES coding_questions(question_id)
        ON DELETE CASCADE,

    input TEXT NOT NULL,
    expected_output TEXT NOT NULL,
    points NUMERIC(6, 2) NOT NULL DEFAULT 1.00,

    position INTEGER NOT NULL,
    is_hidden BOOLEAN NOT NULL DEFAULT TRUE,
    time_limit_override_ms INTEGER,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT coding_test_cases_position_positive
        CHECK (position >= 0),

    CONSTRAINT coding_test_cases_points_positive
        CHECK (points > 0),

    CONSTRAINT coding_test_cases_unique_position
        UNIQUE (question_id, position)
);

CREATE INDEX idx_coding_test_cases_question_id ON coding_test_cases(question_id);

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
