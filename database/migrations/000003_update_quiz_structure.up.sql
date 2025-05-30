-- Drop old columns
ALTER TABLE quizzes
DROP COLUMN answer_options;

ALTER TABLE quizzes
DROP COLUMN correct_answer;

ALTER TABLE quizzes
DROP COLUMN difficulty_level;

ALTER TABLE quizzes
DROP COLUMN sub_category_id;

-- Add new columns
ALTER TABLE quizzes MODIFY question CLOB;

ALTER TABLE quizzes ADD difficulty VARCHAR2 (10) DEFAULT 'easy' NOT NULL;

ALTER TABLE quizzes ADD model_answers JSON NOT NULL;

ALTER TABLE quizzes ADD keywords JSON NOT NULL;

-- Update answers table structure
DROP TABLE answers;

CREATE TABLE
    answers (
        id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        quiz_id NUMBER NOT NULL,
        user_answer CLOB NOT NULL,
        score NUMBER (3, 2) NOT NULL,
        explanation CLOB NOT NULL,
        keyword_matches JSON NOT NULL,
        completeness NUMBER (3, 2) NOT NULL,
        relevance NUMBER (3, 2) NOT NULL,
        accuracy NUMBER (3, 2) NOT NULL,
        answered_at TIMESTAMP
        WITH
            TIME ZONE DEFAULT SYSTIMESTAMP,
            created_at TIMESTAMP
        WITH
            TIME ZONE DEFAULT SYSTIMESTAMP,
            updated_at TIMESTAMP
        WITH
            TIME ZONE DEFAULT SYSTIMESTAMP,
            deleted_at TIMESTAMP
        WITH
            TIME ZONE,
            CONSTRAINT fk_answers_quiz FOREIGN KEY (quiz_id) REFERENCES quizzes (id)
    );