ALTER TABLE exercise_categories DROP COLUMN embedding;
ALTER TABLE exercise_variants DROP COLUMN embedding;
DROP EXTENSION IF EXISTS vector;
