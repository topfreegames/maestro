-- maestro
-- https://github.com/topfreegames/maestro
--
-- Licensed under the MIT license:
-- http://www.opensource.org/licenses/mit-license
-- Copyright © 2017 Top Free Games <backend@tfgco.com>

REVOKE ALL ON SCHEMA public FROM maestro_test;
DROP DATABASE IF EXISTS maestro_test;

DROP ROLE maestro_test;

CREATE ROLE maestro_test LOGIN
  SUPERUSER INHERIT CREATEDB CREATEROLE;

CREATE DATABASE maestro_test
  WITH OWNER = maestro_test
       ENCODING = 'UTF8'
       TABLESPACE = pg_default
       TEMPLATE = template0;

GRANT ALL ON SCHEMA public TO maestro_test;
