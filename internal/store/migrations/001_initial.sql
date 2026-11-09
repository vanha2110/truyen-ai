CREATE TABLE novels (
 id INTEGER PRIMARY KEY, title TEXT NOT NULL CHECK(length(trim(title))>0), author TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', source_url TEXT NOT NULL DEFAULT '',
 last_read_chapter INTEGER NOT NULL DEFAULT 0, last_read_offset INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE chapters (
 id INTEGER PRIMARY KEY, novel_id INTEGER NOT NULL REFERENCES novels(id) ON DELETE CASCADE, idx INTEGER NOT NULL CHECK(idx>0), title TEXT NOT NULL, source_url TEXT NOT NULL DEFAULT '',
 raw_text TEXT NOT NULL DEFAULT '', raw_hash TEXT NOT NULL DEFAULT '', edited_text TEXT NOT NULL DEFAULT '', summary TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','queued','processing','done','needs_review','error')),
 model_used TEXT NOT NULL DEFAULT '', preset_json TEXT NOT NULL DEFAULT '', flags TEXT NOT NULL DEFAULT '', manual_edit INTEGER NOT NULL DEFAULT 0 CHECK(manual_edit IN(0,1)),
 updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(novel_id,idx)
);
CREATE TABLE chunks (
 id INTEGER PRIMARY KEY, chapter_id INTEGER NOT NULL REFERENCES chapters(id) ON DELETE CASCADE, idx INTEGER NOT NULL CHECK(idx>=0), raw_text TEXT NOT NULL,
 edited_text TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','processing','done','needs_review')),
 attempts INTEGER NOT NULL DEFAULT 0, crash_count INTEGER NOT NULL DEFAULT 0, flags TEXT NOT NULL DEFAULT '', started_at TEXT, UNIQUE(chapter_id,idx)
);
CREATE TABLE glossary (
 id INTEGER PRIMARY KEY, novel_id INTEGER REFERENCES novels(id) ON DELETE CASCADE, source_term TEXT NOT NULL CHECK(length(trim(source_term))>0), target_term TEXT NOT NULL CHECK(length(trim(target_term))>0),
 type TEXT NOT NULL DEFAULT 'name', note TEXT NOT NULL DEFAULT '', priority INTEGER NOT NULL DEFAULT 0, locked INTEGER NOT NULL DEFAULT 0 CHECK(locked IN(0,1)), origin TEXT NOT NULL DEFAULT 'manual'
);
CREATE UNIQUE INDEX glossary_scope ON glossary(COALESCE(novel_id,0),source_term);
CREATE TABLE glossary_variants (glossary_id INTEGER NOT NULL REFERENCES glossary(id) ON DELETE CASCADE, variant TEXT NOT NULL, PRIMARY KEY(glossary_id,variant));
CREATE TABLE style_profiles (novel_id INTEGER PRIMARY KEY REFERENCES novels(id) ON DELETE CASCADE, value TEXT NOT NULL);
CREATE TABLE jobs (
 id INTEGER PRIMARY KEY, novel_id INTEGER NOT NULL REFERENCES novels(id) ON DELETE CASCADE, from_idx INTEGER NOT NULL CHECK(from_idx>0), to_idx INTEGER NOT NULL CHECK(to_idx>=from_idx),
 status TEXT NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','paused','interrupted','done','cancelled','error')),
 priority INTEGER NOT NULL DEFAULT 0, error TEXT NOT NULL DEFAULT '', auto_retry INTEGER NOT NULL DEFAULT 0, retry_after INTEGER NOT NULL DEFAULT 0, failures INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, started_at TEXT, finished_at TEXT
);
CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE novel_overrides (novel_id INTEGER PRIMARY KEY REFERENCES novels(id) ON DELETE CASCADE, value TEXT NOT NULL);
CREATE TABLE model_presets (name TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE model_bench (id INTEGER PRIMARY KEY, model_name TEXT NOT NULL, result TEXT NOT NULL, tested_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE reference_pairs (id INTEGER PRIMARY KEY, novel_id INTEGER NOT NULL REFERENCES novels(id) ON DELETE CASCADE, convert_text TEXT NOT NULL, reference_text TEXT NOT NULL, approved INTEGER NOT NULL DEFAULT 0);
CREATE TABLE edit_history (id INTEGER PRIMARY KEY, chapter_id INTEGER NOT NULL REFERENCES chapters(id) ON DELETE CASCADE, previous_text TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE INDEX idx_chunks_status ON chunks(status,chapter_id,idx);
CREATE INDEX idx_chapters_novel ON chapters(novel_id,idx);
CREATE INDEX idx_jobs_status ON jobs(status,priority,id);
