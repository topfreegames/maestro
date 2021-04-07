#
# Copyright (c) 2017 TFG Co <backend@tfgco.com>
# Author: TFG Co <backend@tfgco.com>
#
# Permission is hereby granted, free of charge, to any person obtaining a copy of
# this software and associated documentation files (the "Software"), to deal in
# the Software without restriction, including without limitation the rights to
# use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
# the Software, and to permit persons to whom the Software is furnished to do so,
# subject to the following conditions:
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
# FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
# COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
# IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
# CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
#

FROM golang:1.16.3-buster AS build-env

MAINTAINER TFG Co <backend@tfgco.com>

RUN mkdir -p /app/bin

RUN apt update && apt install --no-install-recommends -y postgresql git make libc-dev gcc
RUN go install github.com/jteeuwen/go-bindata/...

ADD . /go/src/github.com/topfreegames/maestro
RUN cd /go/src/github.com/topfreegames/maestro && \
  make setup && \
  make build && \
  make plugins-linux && \
  make assets && \
  mv bin/maestro /app/maestro && \
  mv bin/grpc.so /app/bin/grpc.so && \
  mv config /app/config && \
  mv scripts /app/scripts && \
  mv migrations /app/migrations && \
  mv Makefile /app/Makefile

FROM debian:buster-slim

RUN apt update && apt install -y ca-certificates openssl

WORKDIR /app

COPY --from=build-env /app/maestro /app/maestro
COPY --from=build-env /app/bin/grpc.so /app/bin/grpc.so
COPY --from=build-env /app/config /app/config
COPY --from=build-env /app/scripts /app/scripts
COPY --from=build-env /app/migrations /app/migrations
COPY --from=build-env /app/Makefile /app/Makefile

EXPOSE 8080

ENV MAESTRO_EXTENSIONS_PG_HOST "maestro-postgres"
ENV MAESTRO_EXTENSIONS_PG_PORT "5432"
ENV MAESTRO_EXTENSIONS_PG_USER "maestro"
ENV MAESTRO_EXTENSIONS_PG_PASS "pass"
ENV PGPASSWORD "pass"

CMD /app/maestro start
