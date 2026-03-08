-- pgvector for semantic exercise matching. Enable extension in Fly dashboard if needed.
CREATE EXTENSION IF NOT EXISTS vector;

-- Embedding dimension for OpenAI text-embedding-3-small
ALTER TABLE exercise_categories ADD COLUMN embedding vector(1536);
ALTER TABLE exercise_variants ADD COLUMN embedding vector(1536);
