-- maestro
-- https://github.com/topfreegames/maestro
--
-- Licensed under the MIT license:
-- http://www.opensource.org/licenses/mit-license
-- Copyright © 2017 Top Free Games <backend@tfgco.com>

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS schedulers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name varchar(255) NOT NULL,
    game varchar(255) NOT NULL,
    yaml TEXT NOT NULL,
    state TEXT NOT NULL,
    state_last_changed_at INTEGER NOT NULL DEFAULT 0,
    last_scale_op_at INTEGER NOT NULL DEFAULT 0,
    created_at timestamp WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at timestamp WITH TIME ZONE NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS schedulers_name_unique ON schedulers (name);
