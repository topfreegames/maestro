-- maestro
-- https://github.com/topfreegames/maestro
--
-- Licensed under the MIT license:
-- http://www.opensource.org/licenses/mit-license
-- Copyright © 2017 Top Free Games <backend@tfgco.com>

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name varchar(255) NOT NULL,
    game varchar(255) NOT NULL,
    yaml TEXT NOT NULL,
    created_at timestamp WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at timestamp WITH TIME ZONE NULL
);

CREATE UNIQUE INDEX configs_name_unique ON configs (name);
