-- maestro
-- https://github.com/topfreegames/maestro
--
-- Licensed under the MIT license:
-- http://www.opensource.org/licenses/mit-license
-- Copyright © 2018 Top Free Games <backend@tfgco.com>

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE scheduler_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name varchar(255) NOT NULL REFERENCES schedulers(name) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    yaml TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX schedulers_version_unique ON scheduler_versions (name, version);
CREATE INDEX scheduler_name ON schedulers (name);
