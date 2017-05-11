Hosting Maestro
===============

## Docker

Maestro needs to connect to a PostgreSQL database in order to persist schedulers configuration and state. The following environment variables must be specified:

* `MAESTRO_EXTENSIONS_PG_HOST` - PostgreSQL host to connect to;
* `MAESTRO_EXTENSIONS_PG_PORT` - PostgreSQL port to connect to;
* `MAESTRO_EXTENSIONS_PG_USER` - User of the PostgreSQL Server to connect to;
* `MAESTRO_EXTENSIONS_PG_PASS` - Password of the PostgreSQL Server to connect to;
* `MAESTRO_EXTENSIONS_PG_POOLSIZE` - PostgreSQL connection pool size;
* `MAESTRO_EXTENSIONS_PG_MAXRETRIES` - PostgreSQL connection max retries;
* `MAESTRO_EXTENSIONS_PG_DATABASE` - PostgreSQL database to connect to;
* `MAESTRO_EXTENSIONS_PG_CONNECTIONTIMEOUT` - Timeout for trying to establish connection;

Maestro also needs to connect to a Redis database in order to persist rooms statuses and lock watcher executions:

* `MAESTRO_EXTENSIONS_REDIS_URL` - Url of the Redis database to connect to;
* `MAESTRO_EXTENSIONS_REDIS_CONNECTIONTIMEOUT` - Timeout for trying to establish connection;

The watcher receives a few environment variables in order to configure itself:

* `MAESTRO_WATCHER_LOCKKEY` - String that will be as key for locking watcher executions;
* `MAESTRO_WATCHER_LOCKTIMEOUT` - Timeout for trying to acquire the watcher lock;
* `MAESTRO_WATCHER_GRACEFULSHUTDOWNTIMEOUT` - Timeout for graceful shutdown;
* `MAESTRO_WATCHER_AUTOSCALINGPERIOD` - Period (in seconds) for executing the watcher autoscaling;

The worker also receives an environment variable for configuration:

* `MAESTRO_WORKER_GRACEFULSHUTDOWNTIMEOUT` - Timeout for graceful shutdown;
* `MAESTRO_WORKER_SYNCPERIOD` - Period (in seconds) for executing the worker scheduler synchronization;

Other than that, there are a couple more configurations you can pass using environment variables:

* `MAESTRO_SCALEUPTIMEOUTSECONDS` - Timeout for trying to scale up a scheduler;
* `MAESTRO_DELETETIMEOUTSECONDS` - Timeout for trying to delete a scheduler;
* `MAESTRO_PINGTIMEOUT` - If a room sent the last PING request more than `MAESTRO_PINGTIMEOUT` seconds ago it is considered unresponsive and removed from the scheduler;

If you wish Sentry integration simply set the following environment variable:

* `MAESTRO_SENTRY_URL` - Sentry Client Key (DSN);

If you wish NewRelic integration you must set the following environment variables:

* `MAESTRO_NEWRELIC_APP` - NewRelic app name;
* `MAESTRO_NEWRELIC_KEY` - NewRelic key;
