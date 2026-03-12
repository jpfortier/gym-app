-- Wipe exercise-related and session data (pre-production reset)
DELETE FROM log_entry_sets;
DELETE FROM personal_records;
DELETE FROM log_entries;
DELETE FROM notes;
DELETE FROM user_exercise_aliases;
DELETE FROM workout_sessions;
DELETE FROM exercise_variants;
DELETE FROM exercise_categories;
DELETE FROM chat_messages;

-- Replace user_exercise_aliases with unified exercise_aliases (user_id NULL = global)
DROP TABLE IF EXISTS user_exercise_aliases;

CREATE TABLE exercise_aliases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    alias_key text NOT NULL,
    variant_id uuid NOT NULL REFERENCES exercise_variants(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_exercise_aliases_global ON exercise_aliases (alias_key) WHERE user_id IS NULL;
CREATE UNIQUE INDEX idx_exercise_aliases_user ON exercise_aliases (user_id, alias_key) WHERE user_id IS NOT NULL;
CREATE INDEX idx_exercise_aliases_lookup ON exercise_aliases (alias_key);

-- Seed global categories
INSERT INTO exercise_categories (user_id, name) VALUES
    (NULL, 'deadlift'),
    (NULL, 'squat'),
    (NULL, 'bench press'),
    (NULL, 'overhead press'),
    (NULL, 'row'),
    (NULL, 'pull up'),
    (NULL, 'dip'),
    (NULL, 'curl'),
    (NULL, 'triceps extension'),
    (NULL, 'lunge'),
    (NULL, 'hip thrust');

-- Seed global variants (with standard=true for default per category)
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'conventional deadlift', true FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'deadlift';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'sumo deadlift', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'deadlift';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'romanian deadlift', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'deadlift';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'rack pull', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'deadlift';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'trap bar deadlift', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'deadlift';

INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'back squat', true FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'squat';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'front squat', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'squat';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'goblet squat', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'squat';

INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'bench press', true FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'bench press';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'close grip bench press', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'bench press';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'incline bench press', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'bench press';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'decline bench press', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'bench press';

INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'overhead press', true FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'overhead press';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'push press', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'overhead press';

INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'barbell row', true FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'row';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'dumbbell row', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'row';

INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'pull up', true FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'pull up';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'chin up', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'pull up';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'neutral grip pull up', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'pull up';

INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'parallel bar dip', true FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'dip';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'weighted dip', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'dip';

INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'barbell curl', true FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'curl';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'hammer curl', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'curl';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'preacher curl', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'curl';

INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'skull crusher', true FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'triceps extension';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'cable pushdown', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'triceps extension';

INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'bulgarian split squat', true FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'lunge';

INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'barbell hip thrust', true FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'hip thrust';
INSERT INTO exercise_variants (category_id, user_id, name, standard)
SELECT c.id, NULL, 'glute bridge', false FROM exercise_categories c WHERE c.user_id IS NULL AND c.name = 'hip thrust';

-- Seed global aliases (alias_key = category + " " + alias, maps to variant)
-- deadlift
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'deadlift rdl', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'deadlift' AND ev.name = 'romanian deadlift';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'deadlift romanian dl', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'deadlift' AND ev.name = 'romanian deadlift';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'deadlift deadlift', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'deadlift' AND ev.name = 'conventional deadlift';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'deadlift sumo dl', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'deadlift' AND ev.name = 'sumo deadlift';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'deadlift rack deadlift', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'deadlift' AND ev.name = 'rack pull';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'deadlift pin pull', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'deadlift' AND ev.name = 'rack pull';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'deadlift hex bar deadlift', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'deadlift' AND ev.name = 'trap bar deadlift';

-- squat
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'squat squat', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'squat' AND ev.name = 'back squat';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'squat front squat', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'squat' AND ev.name = 'front squat';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'squat goblet', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'squat' AND ev.name = 'goblet squat';

-- bench press
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'bench press bench', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'bench press' AND ev.name = 'bench press';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'bench press bench press', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'bench press' AND ev.name = 'bench press';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'bench press cg bench', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'bench press' AND ev.name = 'close grip bench press';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'bench press close grip', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'bench press' AND ev.name = 'close grip bench press';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'bench press incline bench', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'bench press' AND ev.name = 'incline bench press';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'bench press decline bench', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'bench press' AND ev.name = 'decline bench press';

-- overhead press
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'overhead press ohp', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'overhead press' AND ev.name = 'overhead press';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'overhead press military press', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'overhead press' AND ev.name = 'overhead press';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'overhead press push press', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'overhead press' AND ev.name = 'push press';

-- row
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'row row', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'row' AND ev.name = 'barbell row';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'row bent row', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'row' AND ev.name = 'barbell row';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'row db row', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'row' AND ev.name = 'dumbbell row';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'row one arm row', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'row' AND ev.name = 'dumbbell row';

-- pull up
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'pull up pullup', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'pull up' AND ev.name = 'pull up';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'pull up chinup', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'pull up' AND ev.name = 'chin up';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'pull up neutral pullup', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'pull up' AND ev.name = 'neutral grip pull up';

-- dip
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'dip dip', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'dip' AND ev.name = 'parallel bar dip';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'dip weighted dip', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'dip' AND ev.name = 'weighted dip';

-- curl
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'curl curl', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'curl' AND ev.name = 'barbell curl';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'curl hammer', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'curl' AND ev.name = 'hammer curl';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'curl preacher', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'curl' AND ev.name = 'preacher curl';

-- triceps extension
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'triceps extension skullcrusher', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'triceps extension' AND ev.name = 'skull crusher';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'triceps extension skullcrushers', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'triceps extension' AND ev.name = 'skull crusher';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'triceps extension pushdown', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'triceps extension' AND ev.name = 'cable pushdown';

-- lunge (bulgarian split squat)
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'lunge bulgarian', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'lunge' AND ev.name = 'bulgarian split squat';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'lunge split squat', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'lunge' AND ev.name = 'bulgarian split squat';

-- hip thrust
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'hip thrust hip thrust', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'hip thrust' AND ev.name = 'barbell hip thrust';
INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
SELECT NULL, 'hip thrust glute bridge', ev.id FROM exercise_variants ev JOIN exercise_categories ec ON ec.id = ev.category_id WHERE ec.user_id IS NULL AND ec.name = 'hip thrust' AND ev.name = 'glute bridge';
