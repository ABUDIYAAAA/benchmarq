-- Marks given to new questions in the section when the author does not set
-- them. Questions store their own marks; this is only a default.
ALTER TABLE exam_sections
    ADD COLUMN default_points NUMERIC(6, 2),
    ADD CONSTRAINT exam_sections_default_points_positive
        CHECK (default_points IS NULL OR default_points > 0);
