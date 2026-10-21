-- Coding questions and essay file attachments are no longer supported.
-- Existing coding sections are deleted together with their questions.

DROP TABLE attempt_coding_submissions;
DROP TABLE coding_test_cases;
DROP TABLE coding_questions;
DROP TABLE coding_languages;

DELETE FROM exam_sections WHERE section_type = 'coding';

-- Close the gaps the deleted sections left in each exam's ordering.
UPDATE exam_sections s
SET position = o.rn - 1
FROM (
    SELECT id, row_number() OVER (PARTITION BY exam_id ORDER BY position) AS rn
    FROM exam_sections
) o
WHERE s.id = o.id AND s.position <> o.rn - 1;

-- Enum values cannot be dropped in place, so both enums are rebuilt. The
-- constraints that compare these columns are recreated around the swap.
ALTER TABLE questions
    DROP CONSTRAINT question_type_matches_section_family,
    DROP CONSTRAINT questions_section_fk;

ALTER TABLE exam_sections
    DROP CONSTRAINT exam_sections_unique_id_type;

ALTER TYPE section_type RENAME TO section_type_old;
CREATE TYPE section_type AS ENUM (
    'objective',
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
            (section_type = 'subjective' AND type IN ('short_answer', 'essay'))
        );

ALTER TABLE exam_configurations
    DROP COLUMN allowed_language_ids,
    DROP COLUMN code_run_limit_per_question,
    DROP COLUMN editor_autocomplete_enabled,
    DROP COLUMN allow_custom_test_input,
    DROP COLUMN show_failed_testcase_diff,
    DROP COLUMN default_timeout_ms,
    DROP COLUMN default_memory_limit_mb;

ALTER TABLE essay_questions
    DROP COLUMN allow_file_attachments;
