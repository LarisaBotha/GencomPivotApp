DROP TABLE IF EXISTS pivot_command_queue;
DROP TABLE IF EXISTS pivot_status;
DROP TABLE IF EXISTS pivots;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS pivot_system_status;
DROP TYPE IF EXISTS pivot_direction;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE pivot_direction AS ENUM ('forward', 'reverse');
CREATE TYPE pivot_system_status AS ENUM ('running', 'stopped', 'error', 'offline');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cell TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE pivots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    "user" UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    imei TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE pivot_command_queue (
    id SERIAL PRIMARY KEY,
    pivot_id UUID NOT NULL REFERENCES pivots(id) ON DELETE CASCADE,
    command TEXT NOT NULL,
    payload TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    acknowledged BOOLEAN DEFAULT FALSE,
    acknowledged_at TIMESTAMP
);

CREATE TYPE pivot_speed_section AS (
    speed_pct FLOAT,
    label TEXT,
    angle_deg FLOAT
);

CREATE TABLE pivot_status (
    pivot_id UUID PRIMARY KEY REFERENCES pivots(id) ON DELETE CASCADE,
    position_deg FLOAT NOT NULL DEFAULT 0,
    speed_pct FLOAT NOT NULL DEFAULT 0,
    direction pivot_direction NOT NULL DEFAULT 'forward',
    wet BOOLEAN NOT NULL DEFAULT false,
    status pivot_system_status NOT NULL DEFAULT 'offline',
    battery_pct FLOAT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    speed_sections pivot_speed_section[]
);

CREATE OR REPLACE FUNCTION create_pivot_status()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO pivot_status (pivot_id, speed_sections) VALUES (NEW.id, ARRAY[(100.0, NULL::TEXT, 360.0)]::pivot_speed_section[]);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER trg_create_pivot_status
AFTER INSERT ON pivots
FOR EACH ROW
EXECUTE FUNCTION create_pivot_status();

-- CREATE INDEX idx_pivot_command_queue_pivot_id ON pivot_command_queue(pivot_id);
-- CREATE INDEX idx_pivot_command_queue_executed ON pivot_command_queue(acknowledged);
