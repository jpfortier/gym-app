-- Replace is_bodyweight with explicit display flags per field
ALTER TABLE exercise_categories DROP COLUMN is_bodyweight;

ALTER TABLE exercise_categories ADD COLUMN show_weight boolean NOT NULL DEFAULT true;
ALTER TABLE exercise_categories ADD COLUMN show_reps boolean NOT NULL DEFAULT true;

-- Push-up, pull-up, lunge: weight not relevant
UPDATE exercise_categories SET show_weight = false
WHERE user_id IS NULL AND LOWER(name) IN ('push-up', 'pull-up', 'lunge');
