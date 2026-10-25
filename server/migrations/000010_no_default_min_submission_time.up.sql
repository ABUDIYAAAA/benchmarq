-- New exams can be submitted at any time unless the author sets a minimum.
ALTER TABLE exam_configurations
    ALTER COLUMN min_time_before_submission_minutes SET DEFAULT 0;
