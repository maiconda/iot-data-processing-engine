CREATE TABLE IF NOT EXISTS telemetria
(
    data_hora
    TIMESTAMPTZ
    NOT
    NULL
    DEFAULT
    NOW
(
),
    imei TEXT NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    velocidade INTEGER NOT NULL
    );

SELECT create_hypertable('telemetria', 'data_hora');

CREATE INDEX idx_telemetria_imei_tempo ON telemetria (imei, data_hora DESC);