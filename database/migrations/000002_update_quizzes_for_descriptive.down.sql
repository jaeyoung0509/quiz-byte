-- Revert changes to quizzes table
ALTER TABLE quizzes
DROP COLUMN model_answers;

-- Revert changes to answers table
ALTER TABLE answers ADD is_correct NUMBER (1) NOT NULL;

ALTER TABLE answers
DROP COLUMN keyword_matches;

ALTER TABLE answers
DROP COLUMN completeness;

ALTER TABLE answers
DROP COLUMN relevance;

ALTER TABLE answers
DROP COLUMN accuracy;

-- Revert changes to quiz_evaluations table
ALTER TABLE quiz_evaluations ADD score_range VARCHAR2 (20) NOT NULL;

ALTER TABLE quiz_evaluations
DROP COLUMN minimum_keywords;

ALTER TABLE quiz_evaluations
DROP COLUMN required_topics;

ALTER TABLE quiz_evaluations
DROP COLUMN score_ranges;

ALTER TABLE quiz_evaluations
DROP COLUMN sample_answers;

ALTER TABLE quiz_evaluations
DROP COLUMN rubric_details;