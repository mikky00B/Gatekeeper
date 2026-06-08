CREATE TABLE IF NOT EXISTS request_logs (
    id BIGSERIAL PRIMARY KEY,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    route_id TEXT NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL,
    duration_ms BIGINT NOT NULL,
    remote_addr TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_request_logs_created_at ON request_logs (created_at);
CREATE INDEX IF NOT EXISTS idx_request_logs_path_created_at ON request_logs (path, created_at);
CREATE INDEX IF NOT EXISTS idx_request_logs_route_id_created_at ON request_logs (route_id, created_at);
