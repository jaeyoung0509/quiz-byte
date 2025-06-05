-- 000002_add_user_auth_features.down.sql

-- Drop updated_at triggers
DROP TRIGGER user_quiz_attempts_updated_at_trigger;
DROP TRIGGER users_updated_at_trigger;

-- Drop indexes
DROP INDEX idx_user_quiz_attempts_attempted_at;
DROP INDEX idx_user_quiz_attempts_quiz_id;
DROP INDEX idx_user_quiz_attempts_user_id;
DROP INDEX idx_users_email;
DROP INDEX idx_users_google_id;

-- Drop tables
DROP TABLE user_quiz_attempts;
DROP TABLE users;
