-- Restore old columns
ALTER TABLE quizzes ADD answer_options VARCHAR2 (1000);

ALTER TABLE quizzes ADD correct_answer VARCHAR2 (100);

ALTER TABLE quizzes ADD difficulty_level NUMBER (1);

ALTER TABLE quizzes ADD sub_category_id NUMBER;

-- Remove new columns
ALTER TABLE quizzes
DROP COLUMN difficulty;

ALTER TABLE quizzes
DROP COLUMN model_answers;

ALTER TABLE quizzes
DROP COLUMN keywords;

-- Restore answers table to original structure
DROP TABLE answers;

CREATE TABLE
    answers (
        id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        quiz_id NUMBER NOT NULL,
        user_answer VARCHAR2 (100) NOT NULL,
        is_correct NUMBER (1) NOT NULL,
        created_at TIMESTAMP
        WITH
            TIME ZONE DEFAULT SYSTIMESTAMP,
            updated_at TIMESTAMP
        WITH
            TIME ZONE DEFAULT SYSTIMESTAMP,
            CONSTRAINT fk_answers_quiz_old FOREIGN KEY (quiz_id) REFERENCES quizzes (id)
    );