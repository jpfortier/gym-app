ALTER TABLE exercise_categories DROP COLUMN is_bodyweight;
ALTER TABLE log_entry_sets ALTER COLUMN weight SET NOT NULL;
