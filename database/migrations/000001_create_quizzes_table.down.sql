-- Drop triggers
DROP TRIGGER quiz_evaluations_updated_at;

DROP TRIGGER answers_updated_at;

DROP TRIGGER quizzes_updated_at;

DROP TRIGGER sub_categories_updated_at;

DROP TRIGGER categories_updated_at;

-- Drop tables in reverse order of creation
DROP TABLE quiz_evaluations;

DROP TABLE answers;

DROP TABLE quizzes;

DROP TABLE sub_categories;

DROP TABLE categories;