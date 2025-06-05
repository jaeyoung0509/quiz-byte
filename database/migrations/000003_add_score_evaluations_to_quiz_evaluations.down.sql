-- +migrate Down
ALTER TABLE quiz_evaluations
DROP COLUMN score_evaluations;
