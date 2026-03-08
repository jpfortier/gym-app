-- Weight nullable for bodyweight exercises (push-ups, pull-ups, etc.)
ALTER TABLE log_entry_sets ALTER COLUMN weight DROP NOT NULL;

-- Metadata: which fields apply. is_bodyweight=true → don't show weight in UI
ALTER TABLE exercise_categories ADD COLUMN is_bodyweight boolean NOT NULL DEFAULT false;

UPDATE exercise_categories SET is_bodyweight = true
WHERE user_id IS NULL AND LOWER(name) IN ('push-up', 'pull-up', 'lunge');
