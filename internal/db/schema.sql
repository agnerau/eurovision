CREATE TABLE IF NOT EXISTS users (
                                     id BIGSERIAL PRIMARY KEY,
                                     username VARCHAR(255) NOT NULL UNIQUE,
                                     password_hash VARCHAR(255) NOT NULL,
                                     created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS countries (
                                         id BIGSERIAL PRIMARY KEY,
                                         name VARCHAR(255) NOT NULL UNIQUE,
                                         created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS stats (
                                     id BIGSERIAL PRIMARY KEY,
                                     country_id BIGINT NOT NULL,
                                     place INT NOT NULL,
                                     user_id BIGINT NOT NULL,
                                     created_at TIMESTAMP NOT NULL DEFAULT now(),

                                     CONSTRAINT unique_user_country UNIQUE (user_id, country_id),
                                     CONSTRAINT unique_user_place UNIQUE (user_id, place),

                                     CONSTRAINT fk_stats_country
                                         FOREIGN KEY (country_id)
                                             REFERENCES countries(id)
                                             ON DELETE CASCADE,

                                     CONSTRAINT fk_stats_user
                                         FOREIGN KEY (user_id)
                                             REFERENCES users(id)
                                             ON DELETE CASCADE,

                                     CONSTRAINT chk_place_positive CHECK (place > 0)
);

CREATE TABLE IF NOT EXISTS winner_countries (
                                                id BIGSERIAL PRIMARY KEY,
                                                country_id BIGINT NOT NULL,
                                                place INT NOT NULL UNIQUE,
                                                created_at TIMESTAMP NOT NULL DEFAULT now(),

                                                CONSTRAINT fk_winner_country
                                                    FOREIGN KEY (country_id)
                                                        REFERENCES countries(id)
                                                        ON DELETE CASCADE,

                                                CONSTRAINT chk_winner_place_positive CHECK (place > 0)
);
CREATE TABLE IF NOT EXISTS app_settings (
                                            id BOOLEAN PRIMARY KEY DEFAULT TRUE,
                                            predictions_locked BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS eurovision_part (
                                            id BOOLEAN PRIMARY KEY DEFAULT TRUE,
                                            part INT NOT NULL
);