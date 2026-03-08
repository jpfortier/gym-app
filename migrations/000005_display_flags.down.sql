ALTER TABLE exercise_categories DROP COLUMN show_weight;
ALTER TABLE exercise_categories DROP COLUMN show_reps;
ALTER TABLE exercise_categories ADD COLUMN is_bodyweight boolean NOT NULL DEFAULT false;
UPDATE exercise_categories SET is_bodyweight = true
WHERE user_id IS NULL AND LOWER(name) IN ('push-up', 'pull-up', 'lunge');
