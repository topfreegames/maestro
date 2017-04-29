-- maestro
-- https://github.com/topfreegames/maestro
--
-- Licensed under the MIT license:
-- http://www.opensource.org/licenses/mit-license
-- Copyright © 2017 Top Free Games <backend@tfgco.com>

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE rooms (
    id varchar(255) NOT NULL,
    config_id UUID references configs(id) ON DELETE CASCADE,
    status varchar(255) NOT NULL,
    last_ping_at timestamp WITH TIME ZONE NULL,
    created_at timestamp WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at timestamp WITH TIME ZONE NULL
);

CREATE INDEX rooms_config_id ON rooms (config_id);
