-- Exam authoring: section-level overrides, unlock rules and reorder-friendly constraints.

-- Section overrides. A NULL override means "inherit from exam_configurations".
ALTER TABLE exam_sections
    ADD COLUMN shuffle_questions BOOLEAN,
    ADD COLUMN shuffle_options BOOLEAN,
    ADD COLUMN question_pick_count INTEGER,
    ADD COLUMN unlock_after_minutes INTEGER,
    ADD COLUMN lock_on_complete BOOLEAN NOT NULL DEFAULT FALSE,

    ADD CONSTRAINT exam_sections_pick_count_positive
        CHECK (question_pick_count IS NULL OR question_pick_count > 0),

    ADD CONSTRAINT exam_sections_unlock_after_non_negative
        CHECK (unlock_after_minutes IS NULL OR unlock_after_minutes >= 0),

    ADD CONSTRAINT exam_sections_cutoff_non_negative
        CHECK (cutoff_marks IS NULL OR cutoff_marks >= 0);

-- Positions are rewritten in bulk on reorder, so uniqueness is checked at commit.
ALTER TABLE exam_sections
    DROP CONSTRAINT exam_sections_unique_position,
    ADD CONSTRAINT exam_sections_unique_position
        UNIQUE (exam_id, position) DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE questions
    DROP CONSTRAINT questions_unique_position,
    ADD CONSTRAINT questions_unique_position
        UNIQUE (section_id, position) DEFERRABLE INITIALLY IMMEDIATE;

-- A section unlocks only once every prerequisite section is completed
-- (and, when min_score_percent is set, passed with at least that score).
CREATE TABLE exam_section_prerequisites (
    section_id UUID NOT NULL
        REFERENCES exam_sections(id)
        ON DELETE CASCADE,

    prerequisite_section_id UUID NOT NULL
        REFERENCES exam_sections(id)
        ON DELETE CASCADE,

    min_score_percent NUMERIC(5, 2),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (section_id, prerequisite_section_id),

    CONSTRAINT exam_section_prerequisites_not_self
        CHECK (section_id <> prerequisite_section_id),

    CONSTRAINT exam_section_prerequisites_score_range
        CHECK (min_score_percent IS NULL OR (min_score_percent > 0 AND min_score_percent <= 100))
);

CREATE INDEX idx_exam_section_prerequisites_prereq ON exam_section_prerequisites(prerequisite_section_id);

-- Pinned options keep their authored position when options are shuffled
-- (e.g. "All of the above").
ALTER TABLE question_options
    ADD COLUMN is_pinned BOOLEAN NOT NULL DEFAULT FALSE;

-- Answer keys still may not point at a missing option, but the check is
-- deferred to commit. With RESTRICT (or plain NO ACTION) deleting a question
-- or section failed: the cascade reaches question_options before it reaches
-- multi_select_correct_options, which sits one level deeper.
ALTER TABLE multiple_choice_questions
    DROP CONSTRAINT multiple_choice_correct_option_fk,
    ADD CONSTRAINT multiple_choice_correct_option_fk
        FOREIGN KEY (question_id, correct_option_id)
        REFERENCES question_options(question_id, id)
        ON DELETE NO ACTION
        DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE multi_select_correct_options
    DROP CONSTRAINT multi_select_correct_option_fk,
    ADD CONSTRAINT multi_select_correct_option_fk
        FOREIGN KEY (question_id, option_id)
        REFERENCES question_options(question_id, id)
        ON DELETE NO ACTION
        DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE exams
    ADD COLUMN published_at TIMESTAMPTZ;

-- Exam codes only need to be unique among live exams.
ALTER TABLE exams DROP CONSTRAINT exams_unique_org_code;
CREATE UNIQUE INDEX exams_unique_org_code ON exams(org_id, code) WHERE deleted_at IS NULL;

CREATE INDEX idx_exams_org_listing ON exams(org_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_organization_members_user_id ON organization_members(user_id);
