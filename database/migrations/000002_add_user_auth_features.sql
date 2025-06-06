-- +migrate Up
CREATE TABLE users (
    id VARCHAR2(26) PRIMARY KEY,
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
CREATE TABLE user_quiz_attempts (
    id VARCHAR2(26) PRIMARY KEY,
    user_id VARCHAR2(26) NOT NULL,
    quiz_id VARCHAR2(26) NOT NULL,
    user_answer CLOB,
    llm_score NUMBER(5,2),
    llm_explanation CLOB,
    llm_keyword_matches CLOB,
    llm_completeness NUMBER(5,2),
    llm_relevance NUMBER(5,2),
    llm_accuracy NUMBER(5,2),
    is_correct NUMBER(1) DEFAULT 0 NOT NULL,
    attempted_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_user_quiz_attempts_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_quiz_attempts_quiz FOREIGN KEY (quiz_id) REFERENCES quizzes(id) ON DELETE CASCADE
);
CREATE INDEX idx_user_quiz_attempts_user_id ON user_quiz_attempts(user_id);
CREATE INDEX idx_user_quiz_attempts_quiz_id ON user_quiz_attempts(quiz_id);
CREATE INDEX idx_user_quiz_attempts_attempted_at ON user_quiz_attempts(attempted_at);

-- +migrate StatementBegin
CREATE OR REPLACE TRIGGER users_updated_at_trigger
BEFORE UPDATE ON users
FOR EACH ROW
BEGIN
    :NEW.updated_at := SYSTIMESTAMP;
END;
/
-- +migrate StatementEnd

-- +migrate StatementBegin
CREATE OR REPLACE TRIGGER user_quiz_attempts_updated_at_trigger
BEFORE UPDATE ON user_quiz_attempts
FOR EACH ROW
BEGIN
    :NEW.updated_at := SYSTIMESTAMP;
END;
/
-- +migrate StatementEnd

-- +migrate Down
DROP TRIGGER user_quiz_attempts_updated_at_trigger;
DROP TRIGGER users_updated_at_trigger;
DROP INDEX idx_user_quiz_attempts_attempted_at;
DROP INDEX idx_user_quiz_attempts_quiz_id;
DROP INDEX idx_user_quiz_attempts_user_id;
DROP TABLE user_quiz_attempts;
DROP TABLE users;