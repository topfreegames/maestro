-- maestro
-- https://github.com/topfreegames/maestro
--
-- Licensed under the MIT license:
-- http://www.opensource.org/licenses/mit-license
-- Copyright © 2018 Top Free Games <backend@tfgco.com>

ALTER TABLE schedulers ADD COLUMN IF NOT EXISTS version INTEGER;
