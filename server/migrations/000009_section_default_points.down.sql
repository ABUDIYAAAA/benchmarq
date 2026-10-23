ALTER TABLE exam_sections
    DROP CONSTRAINT IF EXISTS exam_sections_default_points_positive,
    DROP COLUMN IF EXISTS default_points;
