-- +migrate Up
ALTER TABLE quiz_evaluations
ADD COLUMN score_evaluations TEXT;
