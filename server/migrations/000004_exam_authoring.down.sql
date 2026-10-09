DROP INDEX IF EXISTS idx_organization_members_user_id;
DROP INDEX IF EXISTS idx_exams_org_listing;

DROP INDEX IF EXISTS exams_unique_org_code;
ALTER TABLE exams ADD CONSTRAINT exams_unique_org_code UNIQUE (org_id, code);

ALTER TABLE exams DROP COLUMN IF EXISTS published_at;

ALTER TABLE multi_select_correct_options
    DROP CONSTRAINT multi_select_correct_option_fk,
    ADD CONSTRAINT multi_select_correct_option_fk
        FOREIGN KEY (question_id, option_id)
        REFERENCES question_options(question_id, id)
        ON DELETE RESTRICT;

ALTER TABLE multiple_choice_questions
    DROP CONSTRAINT multiple_choice_correct_option_fk,
    ADD CONSTRAINT multiple_choice_correct_option_fk
        FOREIGN KEY (question_id, correct_option_id)
        REFERENCES question_options(question_id, id)
        ON DELETE RESTRICT;

ALTER TABLE question_options DROP COLUMN IF EXISTS is_pinned;

DROP TABLE IF EXISTS exam_section_prerequisites;

ALTER TABLE questions
    DROP CONSTRAINT questions_unique_position,
    ADD CONSTRAINT questions_unique_position UNIQUE (section_id, position);

ALTER TABLE exam_sections
    DROP CONSTRAINT exam_sections_unique_position,
    ADD CONSTRAINT exam_sections_unique_position UNIQUE (exam_id, position);

ALTER TABLE exam_sections
    DROP CONSTRAINT IF EXISTS exam_sections_cutoff_non_negative,
    DROP CONSTRAINT IF EXISTS exam_sections_unlock_after_non_negative,
    DROP CONSTRAINT IF EXISTS exam_sections_pick_count_positive,
    DROP COLUMN IF EXISTS lock_on_complete,
    DROP COLUMN IF EXISTS unlock_after_minutes,
    DROP COLUMN IF EXISTS question_pick_count,
    DROP COLUMN IF EXISTS shuffle_options,
    DROP COLUMN IF EXISTS shuffle_questions;
