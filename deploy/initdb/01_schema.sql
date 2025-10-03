
CREATE TABLE IF NOT EXISTS jobs (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    payload TEXT,
    status TEXT DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    lease_start TIMESTAMP,
    lease_timeout INT,
    leased_to_worker TEXT,
    completed_at TIMESTAMP,
    retries INT DEFAULT 0,
    max_retries INT DEFAULT 3,
    result TEXT
);

CREATE TABLE IF NOT EXISTS workers (
    id SERIAL PRIMARY KEY,
    state TEXT DEFAULT 'inactive',
    url TEXT UNIQUE,
    jobs_completed INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
