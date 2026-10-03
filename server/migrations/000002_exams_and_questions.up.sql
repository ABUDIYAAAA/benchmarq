CREATE TYPE exam_status AS ENUM (
    'draft',
    'published',
    'archived',
    'concluded'
);

CREATE TYPE schedule_type AS ENUM (
    'fixed_window',
    'flexible_window',
    'open_ended'
);

CREATE TYPE section_progression_mode AS ENUM (
    'linear_strict',
    'linear_relaxed',
    'free_navigation',
    'timed_sequential'
);

CREATE TYPE question_navigation_mode AS ENUM (
    'free_jump',
    'sequential_only',
    'mandatory_answer'
);

CREATE TYPE multi_select_scoring_mode AS ENUM (
    'all_or_nothing',
    'partial_with_penalty',
    'partial_no_penalty'
);

CREATE TYPE proctoring_mode AS ENUM (
    'disabled',
    'browser_only',
    'ai_proctored',
    'live_proctored'
);

CREATE TYPE calculator_type AS ENUM (
    'none',
    'basic',
    'scientific'
);

CREATE TYPE results_release_mode AS ENUM (
    'instant',
    'scheduled',
    'manual'
);

CREATE TYPE solution_visibility AS ENUM (
    'never',
    'after_submission',
    'after_exam_closes'
);

CREATE TYPE retake_score_rule AS ENUM (
    'best_attempt',
    'latest_attempt',
    'average_attempt'
);

CREATE TYPE section_type AS ENUM (
    'objective',
    'coding',
    'subjective'
);

CREATE TYPE question_type AS ENUM (
    'multiple_choice',
    'true_false',
    'multi_select',
    'blank_fill',
    'coding',
    'short_answer',
    'essay'
);

-- Exams Core Table
CREATE TABLE exams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    org_id UUID NOT NULL
        REFERENCES organizations(id)
        ON DELETE CASCADE,

    created_by_user_id UUID NOT NULL
        REFERENCES users(id)
        ON DELETE RESTRICT,

    title VARCHAR(255) NOT NULL,
    code VARCHAR(50),
    description TEXT,
    instructions TEXT,

    status exam_status NOT NULL DEFAULT 'draft',
    total_marks NUMERIC(8, 2) NOT NULL DEFAULT 0.00,
    passing_marks NUMERIC(8, 2),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT exams_unique_org_code
        UNIQUE (org_id, code)
);

CREATE INDEX idx_exams_org_id ON exams(org_id);
CREATE INDEX idx_exams_status ON exams(status);

-- Exhaustive 9-Category Exam Configurations
CREATE TABLE exam_configurations (
    exam_id UUID PRIMARY KEY
        REFERENCES exams(id)
        ON DELETE CASCADE,

    -- Category 1: Schedule & Timing Rules
    schedule_type schedule_type NOT NULL DEFAULT 'flexible_window',
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    total_duration_minutes INTEGER NOT NULL DEFAULT 60,
    per_section_timing BOOLEAN NOT NULL DEFAULT FALSE,
    per_question_timing BOOLEAN NOT NULL DEFAULT FALSE,
    late_join_grace_minutes INTEGER NOT NULL DEFAULT 15,
    min_time_before_submission_minutes INTEGER NOT NULL DEFAULT 5,
    auto_submit_on_expiry BOOLEAN NOT NULL DEFAULT TRUE,
    allow_candidate_pause BOOLEAN NOT NULL DEFAULT FALSE,
    max_pause_count INTEGER NOT NULL DEFAULT 0,
    max_total_pause_minutes INTEGER NOT NULL DEFAULT 0,
    reconnection_grace_seconds INTEGER NOT NULL DEFAULT 120,

    -- Category 2: Navigation & Section Progression Rules
    section_progression_mode section_progression_mode NOT NULL DEFAULT 'free_navigation',
    question_navigation_mode question_navigation_mode NOT NULL DEFAULT 'free_jump',
    allow_revisiting_questions BOOLEAN NOT NULL DEFAULT TRUE,
    allow_bookmark_for_review BOOLEAN NOT NULL DEFAULT TRUE,
    auto_advance_on_selection BOOLEAN NOT NULL DEFAULT FALSE,

    -- Category 3: Randomization & Question Pooling Rules
    shuffle_sections BOOLEAN NOT NULL DEFAULT FALSE,
    shuffle_questions BOOLEAN NOT NULL DEFAULT FALSE,
    shuffle_options BOOLEAN NOT NULL DEFAULT FALSE,
    enable_question_pool BOOLEAN NOT NULL DEFAULT FALSE,
    pool_sample_count INTEGER,
    deterministic_user_seed BOOLEAN NOT NULL DEFAULT TRUE,

    -- Category 4: Scoring, Grading & Negative Marking Policy
    negative_marking_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    negative_marking_penalty_percent NUMERIC(5, 2) NOT NULL DEFAULT 25.00,
    multi_select_scoring_mode multi_select_scoring_mode NOT NULL DEFAULT 'partial_with_penalty',
    passing_percentage NUMERIC(5, 2) NOT NULL DEFAULT 50.00,
    grading_strategy VARCHAR(50) NOT NULL DEFAULT 'automated',
    blind_grading_enabled BOOLEAN NOT NULL DEFAULT FALSE,

    -- Category 5: Proctoring, Anti-Cheating & Security Rules
    proctoring_mode proctoring_mode NOT NULL DEFAULT 'disabled',
    enforce_fullscreen BOOLEAN NOT NULL DEFAULT FALSE,
    max_fullscreen_exit_count INTEGER NOT NULL DEFAULT 3,
    track_tab_focus BOOLEAN NOT NULL DEFAULT TRUE,
    max_tab_switch_count INTEGER NOT NULL DEFAULT 3,
    tab_switch_grace_seconds INTEGER NOT NULL DEFAULT 3,
    block_copy_paste_clipboard BOOLEAN NOT NULL DEFAULT TRUE,
    block_developer_tools_keys BOOLEAN NOT NULL DEFAULT TRUE,
    webcam_snapshots_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    webcam_snapshot_interval_seconds INTEGER NOT NULL DEFAULT 60,
    audio_monitoring_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    multiple_face_detection_flag BOOLEAN NOT NULL DEFAULT FALSE,
    no_face_detection_flag BOOLEAN NOT NULL DEFAULT FALSE,
    ip_whitelist_cidr TEXT[],
    single_session_lock BOOLEAN NOT NULL DEFAULT TRUE,
    require_safe_exam_browser BOOLEAN NOT NULL DEFAULT FALSE,
    seb_config_key VARCHAR(255),

    -- Category 6: Candidate UX & Accessibility
    show_countdown_timer BOOLEAN NOT NULL DEFAULT TRUE,
    timer_warning_thresholds_minutes INTEGER[] NOT NULL DEFAULT '{15, 5, 1}',
    show_progress_indicator BOOLEAN NOT NULL DEFAULT TRUE,
    calculator_type calculator_type NOT NULL DEFAULT 'none',
    scratchpad_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    formula_sheet_url TEXT,
    dark_mode_allowed BOOLEAN NOT NULL DEFAULT TRUE,

    -- Category 7: Coding Sandbox & Compiler Rules
    allowed_language_ids UUID[],
    code_run_limit_per_question INTEGER NOT NULL DEFAULT 30,
    editor_autocomplete_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    allow_custom_test_input BOOLEAN NOT NULL DEFAULT TRUE,
    show_failed_testcase_diff BOOLEAN NOT NULL DEFAULT TRUE,
    default_timeout_ms INTEGER NOT NULL DEFAULT 2000,
    default_memory_limit_mb INTEGER NOT NULL DEFAULT 256,

    -- Category 8: Results, Solutions & Feedback Visibility
    results_release_mode results_release_mode NOT NULL DEFAULT 'instant',
    results_scheduled_time TIMESTAMPTZ,
    solution_visibility solution_visibility NOT NULL DEFAULT 'after_submission',
    show_question_breakdown BOOLEAN NOT NULL DEFAULT TRUE,
    show_class_percentile BOOLEAN NOT NULL DEFAULT FALSE,
    certificate_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    certificate_min_score_percent NUMERIC(5, 2) DEFAULT 70.00,

    -- Category 9: Retake & Attempt Lifecycle Policies
    max_attempts INTEGER NOT NULL DEFAULT 1,
    attempt_cooldown_hours INTEGER NOT NULL DEFAULT 0,
    retake_score_rule retake_score_rule NOT NULL DEFAULT 'best_attempt',
    allow_resume_on_disconnect BOOLEAN NOT NULL DEFAULT TRUE,

    -- JSON Extension Hook for Custom Plugins / Dynamic Rules
    custom_settings JSONB NOT NULL DEFAULT '{}'::jsonb,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT exam_configurations_duration_positive
        CHECK (total_duration_minutes > 0),

    CONSTRAINT exam_configurations_min_exit_positive
        CHECK (min_time_before_submission_minutes >= 0),

    CONSTRAINT exam_configurations_passing_percent_range
        CHECK (passing_percentage >= 0 AND passing_percentage <= 100),

    CONSTRAINT exam_configurations_max_attempts_positive
        CHECK (max_attempts >= 1)
);

-- Exam Sections with Strict Category Enforcement
CREATE TABLE exam_sections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    exam_id UUID NOT NULL
        REFERENCES exams(id)
        ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    description TEXT,
    instructions TEXT,

    section_type section_type NOT NULL,
    position INTEGER NOT NULL,

    duration_minutes INTEGER,
    marks_weightage NUMERIC(6, 2),
    cutoff_marks NUMERIC(6, 2),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT exam_sections_position_positive
        CHECK (position >= 0),

    CONSTRAINT exam_sections_duration_positive
        CHECK (duration_minutes IS NULL OR duration_minutes > 0),

    CONSTRAINT exam_sections_unique_position
        UNIQUE (exam_id, position),

    CONSTRAINT exam_sections_unique_id_type
        UNIQUE (id, section_type)
);

CREATE INDEX idx_exam_sections_exam_id ON exam_sections(exam_id);

-- Questions Table with Strict Section Family Integrity
CREATE TABLE questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    section_id UUID NOT NULL,
    section_type section_type NOT NULL,
    type question_type NOT NULL,

    prompt TEXT NOT NULL,
    explanation TEXT,

    points NUMERIC(6, 2) NOT NULL DEFAULT 1.00,
    negative_points NUMERIC(6, 2) NOT NULL DEFAULT 0.00,

    position INTEGER NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT questions_position_positive
        CHECK (position >= 0),

    CONSTRAINT questions_points_positive
        CHECK (points > 0),

    CONSTRAINT questions_negative_points_non_negative
        CHECK (negative_points >= 0),

    CONSTRAINT questions_unique_position
        UNIQUE (section_id, position),

    CONSTRAINT questions_section_fk
        FOREIGN KEY (section_id, section_type)
        REFERENCES exam_sections(id, section_type)
        ON DELETE CASCADE,

    CONSTRAINT question_type_matches_section_family
        CHECK (
            (section_type = 'objective' AND type IN ('multiple_choice', 'true_false', 'multi_select', 'blank_fill')) OR
            (section_type = 'coding' AND type = 'coding') OR
            (section_type = 'subjective' AND type IN ('short_answer', 'essay'))
        )
);

CREATE INDEX idx_questions_section_id ON questions(section_id);
CREATE INDEX idx_questions_type ON questions(type);

-- Question Options (for Objective Questions)
CREATE TABLE question_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    question_id UUID NOT NULL
        REFERENCES questions(id)
        ON DELETE CASCADE,

    content TEXT NOT NULL,
    position INTEGER NOT NULL,
    is_correct BOOLEAN NOT NULL DEFAULT FALSE,
    weight NUMERIC(5, 2) NOT NULL DEFAULT 1.00,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT question_options_position_positive
        CHECK (position >= 0),

    CONSTRAINT question_options_unique_position
        UNIQUE (question_id, position),

    CONSTRAINT question_options_unique_question_id_id
        UNIQUE (question_id, id)
);

CREATE INDEX idx_question_options_question_id ON question_options(question_id);

-- Objective: Multiple Choice
CREATE TABLE multiple_choice_questions (
    question_id UUID PRIMARY KEY
        REFERENCES questions(id)
        ON DELETE CASCADE,

    correct_option_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT multiple_choice_correct_option_fk
        FOREIGN KEY (question_id, correct_option_id)
        REFERENCES question_options(question_id, id)
        ON DELETE RESTRICT
);

-- Objective: True / False
CREATE TABLE true_false_questions (
    question_id UUID PRIMARY KEY
        REFERENCES questions(id)
        ON DELETE CASCADE,

    correct_answer BOOLEAN NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Objective: Multi-Select
CREATE TABLE multi_select_questions (
    question_id UUID PRIMARY KEY
        REFERENCES questions(id)
        ON DELETE CASCADE,

    min_choices_required INTEGER NOT NULL DEFAULT 1,
    max_choices_allowed INTEGER,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT multi_select_min_choices_positive
        CHECK (min_choices_required >= 1),

    CONSTRAINT multi_select_max_choices_valid
        CHECK (max_choices_allowed IS NULL OR max_choices_allowed >= min_choices_required)
);

CREATE TABLE multi_select_correct_options (
    question_id UUID NOT NULL,
    option_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (question_id, option_id),

    CONSTRAINT multi_select_question_fk
        FOREIGN KEY (question_id)
        REFERENCES multi_select_questions(question_id)
        ON DELETE CASCADE,

    CONSTRAINT multi_select_correct_option_fk
        FOREIGN KEY (question_id, option_id)
        REFERENCES question_options(question_id, id)
        ON DELETE RESTRICT
);

-- Objective: Fill in the Blank
CREATE TABLE blank_fill_questions (
    question_id UUID PRIMARY KEY
        REFERENCES questions(id)
        ON DELETE CASCADE,

    correct_answer TEXT NOT NULL,
    case_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
    alternate_answers TEXT[] NOT NULL DEFAULT '{}'::text[],

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Subjective: Short Answer
CREATE TABLE short_answer_questions (
    question_id UUID PRIMARY KEY
        REFERENCES questions(id)
        ON DELETE CASCADE,

    sample_answer TEXT,
    max_length INTEGER,
    max_words INTEGER,
    rubric_keywords TEXT[] NOT NULL DEFAULT '{}'::text[],

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT short_answer_max_length_positive
        CHECK (max_length IS NULL OR max_length > 0),

    CONSTRAINT short_answer_max_words_positive
        CHECK (max_words IS NULL OR max_words > 0)
);

-- Subjective: Long Essay
CREATE TABLE essay_questions (
    question_id UUID PRIMARY KEY
        REFERENCES questions(id)
        ON DELETE CASCADE,

    rubric_guidelines TEXT,
    min_words INTEGER,
    max_words INTEGER,
    max_length INTEGER,
    allow_rich_text BOOLEAN NOT NULL DEFAULT TRUE,
    allow_file_attachments BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT essay_min_words_positive
        CHECK (min_words IS NULL OR min_words >= 0),

    CONSTRAINT essay_max_words_positive
        CHECK (max_words IS NULL OR (min_words IS NOT NULL AND max_words >= min_words) OR max_words > 0),

    CONSTRAINT essay_max_length_positive
        CHECK (max_length IS NULL OR max_length > 0)
);

-- Coding Environments & Languages
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

-- Coding Questions
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

-- Coding Test Cases
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
