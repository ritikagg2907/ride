CREATE TABLE IF NOT EXISTS drivers (
    id            UUID PRIMARY KEY,
    name          VARCHAR(200) NOT NULL,
    email         VARCHAR(200) NOT NULL UNIQUE,
    phone         VARCHAR(50)  NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    vehicle_type  VARCHAR(50)  NOT NULL DEFAULT 'sedan',
    license_plate VARCHAR(20),
    status        VARCHAR(20)  NOT NULL DEFAULT 'available',
    rating        DOUBLE PRECISION NOT NULL DEFAULT 5.0,
    rating_count  INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_drivers_email  ON drivers(email);
CREATE INDEX IF NOT EXISTS idx_drivers_status ON drivers(status);
