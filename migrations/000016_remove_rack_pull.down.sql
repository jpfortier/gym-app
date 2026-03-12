-- Restore Rack Pull (reverses removal).
INSERT INTO exercise_categories (user_id, name)
SELECT NULL, 'Rack Pull' FROM (VALUES (1)) AS t(x)
WHERE NOT EXISTS (SELECT 1 FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'Rack Pull');

INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'standard', true FROM exercise_categories c
WHERE c.user_id IS NULL AND c.name = 'Rack Pull'
AND NOT EXISTS (
    SELECT 1 FROM exercise_variants v
    WHERE v.category_id = c.id AND v.user_id IS NULL AND v.name = 'standard'
);
