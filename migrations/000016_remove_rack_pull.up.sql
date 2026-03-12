-- Remove Rack Pull as standalone category (it's a variant of deadlift, not its own category).
DELETE FROM exercise_variants WHERE category_id IN (SELECT id FROM exercise_categories WHERE user_id IS NULL AND name = 'Rack Pull');
DELETE FROM exercise_categories WHERE user_id IS NULL AND name = 'Rack Pull';
