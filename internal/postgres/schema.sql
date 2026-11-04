CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS files (
    id BIGSERIAL PRIMARY KEY,
    owner_id BIGINT NOT NULL REFERENCES users(id),
    filename TEXT NOT NULL,
    location TEXT NOT NULL,
    size BIGINT NOT NULL CHECK(size >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE(owner_id,location)
);
CREATE TABLE IF NOT EXISTS operations (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMPTZ,
    operation_type TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK(status IN ('pending','success','failed')),
    error TEXT NOT NULL DEFAULT '',
    file_id BIGINT REFERENCES files(id),
    user_id BIGINT REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS operations_user_id ON operations(user_id,id DESC);
CREATE OR REPLACE FUNCTION kfm_record_result(
    operation_id BIGINT, owner BIGINT, target BIGINT, result TEXT, message TEXT
) RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
    UPDATE operations SET file_id=target,status=result,error=message,finished_at=CURRENT_TIMESTAMP
    WHERE id=operation_id AND user_id IS NOT DISTINCT FROM NULLIF(owner,0) AND status='pending';
    IF NOT FOUND THEN RAISE EXCEPTION 'operation not found'; END IF;
END;
$$;
