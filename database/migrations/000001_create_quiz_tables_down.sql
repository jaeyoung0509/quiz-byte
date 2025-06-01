-- Drop triggers first
DROP TRIGGER quiz_evaluations_updated_at;
DROP TRIGGER answers_updated_at;
DROP TRIGGER quizzes_updated_at;
DROP TRIGGER sub_categories_updated_at;
DROP TRIGGER categories_updated_at;

-- Drop indexes
DROP INDEX idx_quiz_evaluations_quiz_id;
DROP INDEX idx_answers_quiz_id;
DROP INDEX idx_quizzes_difficulty;
DROP INDEX idx_quizzes_sub_category_id;
DROP INDEX idx_sub_categories_category_id;
DROP INDEX idx_categories_name;

-- Drop tables in reverse order (to handle foreign key constraints)
DROP TABLE quiz_evaluations;
DROP TABLE answers;
DROP TABLE quizzes;
DROP TABLE sub_categories;
DROP TABLE categories;
