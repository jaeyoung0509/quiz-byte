-- Modify quizzes table
ALTER TABLE quizzes ADD model_answers CLOB NOT NULL;

-- Modify answers table
ALTER TABLE answers
DROP COLUMN is_correct;

ALTER TABLE answers ADD keyword_matches CLOB NOT NULL;

ALTER TABLE answers ADD completeness NUMBER (3, 2) NOT NULL;

ALTER TABLE answers ADD relevance NUMBER (3, 2) NOT NULL;

ALTER TABLE answers ADD accuracy NUMBER (3, 2) NOT NULL;

-- Modify quiz_evaluations table
ALTER TABLE quiz_evaluations
DROP COLUMN score_range;

ALTER TABLE quiz_evaluations ADD minimum_keywords NUMBER (3) NOT NULL;

ALTER TABLE quiz_evaluations ADD required_topics CLOB NOT NULL;

ALTER TABLE quiz_evaluations ADD score_ranges CLOB NOT NULL;

ALTER TABLE quiz_evaluations ADD sample_answers CLOB NOT NULL;

ALTER TABLE quiz_evaluations ADD rubric_details CLOB NOT NULL;