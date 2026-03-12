-- Add visual_cues to exercise_variants (describes what the exercise looks like for image gen, etc.)
ALTER TABLE exercise_variants ADD COLUMN visual_cues text;

-- Seed visual_cues for global variants from samples/exercises.json
UPDATE exercise_variants ev
SET visual_cues = v.visual_cues
FROM (VALUES
    ('deadlift', 'conventional deadlift', 'barbell on floor near shins' || E'\n' || 'lifter hip hinge' || E'\n' || 'bar pulled to standing'),
    ('deadlift', 'sumo deadlift', 'very wide stance' || E'\n' || 'hands inside knees' || E'\n' || 'barbell pulled from floor'),
    ('deadlift', 'romanian deadlift', 'bar starts at hips' || E'\n' || 'slides down thighs' || E'\n' || 'hip hinge movement'),
    ('deadlift', 'rack pull', 'bar on rack pins' || E'\n' || 'power rack visible' || E'\n' || 'short deadlift range'),
    ('deadlift', 'trap bar deadlift', 'hexagonal trap bar around lifter' || E'\n' || 'hands at sides'),
    ('squat', 'back squat', 'barbell on upper back' || E'\n' || 'lifter descending into squat'),
    ('squat', 'front squat', 'bar on front shoulders' || E'\n' || 'elbows high'),
    ('squat', 'goblet squat', 'dumbbell at chest' || E'\n' || 'deep squat'),
    ('bench press', 'bench press', 'lying on bench' || E'\n' || 'barbell pressed from chest'),
    ('bench press', 'close grip bench press', 'hands close on bar' || E'\n' || 'press from chest'),
    ('bench press', 'incline bench press', 'incline bench' || E'\n' || 'press upward'),
    ('bench press', 'decline bench press', 'decline bench angle'),
    ('overhead press', 'overhead press', 'barbell pressed overhead from shoulders'),
    ('overhead press', 'push press', 'knee dip then explosive press'),
    ('row', 'barbell row', 'torso bent forward' || E'\n' || 'bar pulled to stomach'),
    ('row', 'dumbbell row', 'one knee on bench' || E'\n' || 'row dumbbell'),
    ('pull up', 'pull up', 'wide grip' || E'\n' || 'chin above bar'),
    ('pull up', 'chin up', 'underhand grip' || E'\n' || 'body pulled upward'),
    ('pull up', 'neutral grip pull up', 'neutral handles pull up'),
    ('dip', 'parallel bar dip', 'body between dip bars' || E'\n' || 'press upward'),
    ('dip', 'weighted dip', 'weight plate hanging from belt'),
    ('curl', 'barbell curl', 'barbell curled upward'),
    ('curl', 'hammer curl', 'dumbbells vertical during curl'),
    ('curl', 'preacher curl', 'arms on preacher bench'),
    ('triceps extension', 'skull crusher', 'lying on bench' || E'\n' || 'bar lowered toward forehead'),
    ('triceps extension', 'cable pushdown', 'cable bar pushed downward'),
    ('lunge', 'bulgarian split squat', 'rear foot on bench'),
    ('hip thrust', 'barbell hip thrust', 'shoulders on bench' || E'\n' || 'barbell across hips'),
    ('hip thrust', 'glute bridge', 'lying on floor' || E'\n' || 'barbell across hips')
) AS v(cat_name, var_name, visual_cues)
JOIN exercise_categories ec ON ec.name = v.cat_name AND ec.user_id IS NULL
WHERE ev.category_id = ec.id AND ev.user_id IS NULL AND ev.name = v.var_name;
