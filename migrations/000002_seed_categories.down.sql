DELETE FROM exercise_variants WHERE user_id IS NULL AND name = 'standard'
AND category_id IN (SELECT id FROM exercise_categories WHERE user_id IS NULL);
DELETE FROM exercise_categories WHERE user_id IS NULL;
