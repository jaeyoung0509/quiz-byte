-- Create categories table
CREATE TABLE categories (
    id VARCHAR2(26) PRIMARY KEY,
    name VARCHAR2(100) NOT NULL UNIQUE,
    description VARCHAR2(500),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create sub_categories table
CREATE TABLE sub_categories (
    id VARCHAR2(26) PRIMARY KEY,
    category_id VARCHAR2(26) NOT NULL,
    name VARCHAR2(100) NOT NULL,
    description VARCHAR2(500),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_sub_categories_category FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Create quizzes table
CREATE TABLE quizzes (
    id VARCHAR2(26) PRIMARY KEY,
    question CLOB NOT NULL,
    model_answers CLOB NOT NULL,
    keywords CLOB NOT NULL,
    difficulty NUMBER(1) NOT NULL,
    sub_category_id VARCHAR2(26) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_quizzes_sub_category FOREIGN KEY (sub_category_id) REFERENCES sub_categories(id)
);

-- Create answers table
CREATE TABLE answers (
    id VARCHAR2(26) PRIMARY KEY,
    quiz_id VARCHAR2(26) NOT NULL,
    user_answer CLOB NOT NULL,
    score NUMBER(3,2) NOT NULL,
    explanation CLOB NOT NULL,
    keyword_matches CLOB, -- StringSlice 타입을 위한 CLOB
    completeness NUMBER(3,2) NOT NULL,
    relevance NUMBER(3,2) NOT NULL,
    accuracy NUMBER(3,2) NOT NULL,
    answered_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_answers_quiz FOREIGN KEY (quiz_id) REFERENCES quizzes(id)
);

-- Create indexes
CREATE INDEX idx_categories_name ON categories(name);
CREATE INDEX idx_sub_categories_category_id ON sub_categories(category_id);
CREATE INDEX idx_quizzes_sub_category_id ON quizzes(sub_category_id);
CREATE INDEX idx_quizzes_difficulty ON quizzes(difficulty);
CREATE INDEX idx_answers_quiz_id ON answers(quiz_id);

-- Create updated_at triggers
CREATE OR REPLACE TRIGGER categories_updated_at
BEFORE UPDATE ON categories
FOR EACH ROW
BEGIN
    :NEW.updated_at := SYSTIMESTAMP;
END;
/

CREATE OR REPLACE TRIGGER sub_categories_updated_at
BEFORE UPDATE ON sub_categories
FOR EACH ROW
BEGIN
    :NEW.updated_at := SYSTIMESTAMP;
END;
/

CREATE OR REPLACE TRIGGER quizzes_updated_at
BEFORE UPDATE ON quizzes
FOR EACH ROW
BEGIN
    :NEW.updated_at := SYSTIMESTAMP;
END;
/

CREATE OR REPLACE TRIGGER answers_updated_at
BEFORE UPDATE ON answers
FOR EACH ROW
BEGIN
    :NEW.updated_at := SYSTIMESTAMP;
END;
/
