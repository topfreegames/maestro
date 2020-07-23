Authentication and Authorization
========

## Overview
Maestro supports multiple ways to authenticate and authorize users.

Maestro supports Basic Auth, OAuth and also has support for delegating authentication to [Will.IAM][william]

### Basic Auth

To enable Basic Auth support on maestro you need to pass a non empty username and password in the config, eg:
```
basicauth:
    username: myuser
    password: mypassword
    tryOauthIfUnset: true
```

If `tryOathIfUnset` is true `maestro` will try to authenticate with `oauth` or [Will.IAM][william] when basic auth is missing.

### Oauth

Example config with `oauth` enabled:
```
oauth:
    enabled: true
    acceptedDomains: "mydomain.com" // comma seperated list of accepted domains
```

Oauth is enabled by default, you also need to set the following environment variables to be able use oauth with google:
* `MAESTRO_GOOGLE_CLIENT_ID`
* `MAESTRO_GOOGLE_CLIENT_SECRET`

When using `Oauth` authorization is configured on a maestro level by setting a list of emails in the path `admin.users` in the config.
And on scheduler lever by passing a list of emails in `authorizedUsers` key of scheduler's yaml. 

### William

Example config with support for [Will.IAM][william] enabled:
```
william:
    enabled: true
    url: mywilliamserver.mydomain.com:8080
    iamName: maestro // service name registered on william
    region: us // region for maestro 
```

`maestro` will use the following permission with [Will.IAM][william]:
* `ListSchedulers::{region}::{game}`
* `CreateScheduler::{region}`
* `GetScheduler::{region}::{game}::{scheduler}`
* `UpdateScheduler::{region}::{game}::{scheduler}`
* `ScaleScheduler::{region}::{game}::{scheduler}`
* `DeleteScheduler::{region}::{game}::{scheduler}`

When [Will.IAM][william] is enabled `maestro` will use the Bearer token to check for permissions on the configured url.
If [Will.IAM][william] and `oauth` are enabled then only [Will.IAM][william] will work.

[william]: https://github.com/topfreegames/will.iam
