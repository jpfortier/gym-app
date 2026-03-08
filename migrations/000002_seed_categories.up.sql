-- Global exercise categories (user_id null) with standard variant
INSERT INTO exercise_categories (user_id, name)
SELECT NULL, name FROM (VALUES
    ('Bench Press'),
    ('Deadlift'),
    ('Squat'),
    ('Overhead Press'),
    ('Row'),
    ('Pull-up'),
    ('Push-up'),
    ('Lunge'),
    ('Calf Raise'),
    ('Curl'),
    ('Tricep Extension')
) AS t(name)
WHERE NOT EXISTS (SELECT 1 FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = t.name);

-- Add standard variant for each global category
INSERT INTO exercise_variants (category_id, user_id, name)
SELECT c.id, NULL, 'standard' FROM exercise_categories c
WHERE c.user_id IS NULL
AND NOT EXISTS (
    SELECT 1 FROM exercise_variants v
    WHERE v.category_id = c.id AND v.user_id IS NULL AND v.name = 'standard'
);
