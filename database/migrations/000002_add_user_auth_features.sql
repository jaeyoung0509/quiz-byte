-- 000002_add_user_auth_features.sql

-- Create users table
CREATE TABLE users (
    id VARCHAR2(26) PRIMARY KEY, -- ULID
    google_id VARCHAR2(255) NOT NULL UNIQUE,
    email VARCHAR2(255) NOT NULL UNIQUE,
    name VARCHAR2(255),
    profile_picture_url VARCHAR2(2048),
    encrypted_access_token CLOB,
    encrypted_refresh_token CLOB,
    token_expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create user_quiz_attempts table
CREATE TABLE user_quiz_attempts (
    id VARCHAR2(26) PRIMARY KEY, -- ULID
    user_id VARCHAR2(26) NOT NULL,
    quiz_id VARCHAR2(26) NOT NULL,
    user_answer CLOB,
    llm_score NUMBER(5,2), -- Adjusted precision to allow scores like 100.00 or standard 0.00-1.00 depending on scale
    llm_explanation CLOB,
    llm_keyword_matches CLOB,
    llm_completeness NUMBER(5,2), -- Assuming similar scale
    llm_relevance NUMBER(5,2),   -- Assuming similar scale
    llm_accuracy NUMBER(5,2),    -- Assuming similar scale
    is_correct NUMBER(1) DEFAULT 0 NOT NULL, -- 0 for false, 1 for true
    attempted_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_user_quiz_attempts_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_quiz_attempts_quiz FOREIGN KEY (quiz_id) REFERENCES quizzes(id) ON DELETE CASCADE
);

-- Create indexes
CREATE INDEX idx_users_google_id ON users(google_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_user_quiz_attempts_user_id ON user_quiz_attempts(user_id);
CREATE INDEX idx_user_quiz_attempts_quiz_id ON user_quiz_attempts(quiz_id);
CREATE INDEX idx_user_quiz_attempts_attempted_at ON user_quiz_attempts(attempted_at);

-- Create updated_at triggers for new tables
CREATE OR REPLACE TRIGGER users_updated_at_trigger
BEFORE UPDATE ON users
FOR EACH ROW
BEGIN
    :NEW.updated_at := SYSTIMESTAMP;
END;
/

CREATE OR REPLACE TRIGGER user_quiz_attempts_updated_at_trigger
BEFORE UPDATE ON user_quiz_attempts
FOR EACH ROW
BEGIN
    :NEW.updated_at := SYSTIMESTAMP;
END;
/

-- Note: The existing 'answers' table is not modified here.
-- New authenticated attempts will go into 'user_quiz_attempts'.
-- The 'answers' table will be deprecated for authenticated users as per plan.
