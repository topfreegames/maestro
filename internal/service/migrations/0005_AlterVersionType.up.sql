-- maestro
-- https://github.com/topfreegames/maestro
--
-- Licensed under the MIT license:
-- http://www.opensource.org/licenses/mit-license
-- Copyright © 2018 Top Free Games <backend@tfgco.com>

ALTER TABLE schedulers ALTER COLUMN version TYPE varchar(255);
ALTER TABLE scheduler_versions ALTER COLUMN version TYPE varchar(255);
