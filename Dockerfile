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

FROM golang:1.8-alpine

MAINTAINER TFG Co <backend@tfgco.com>

RUN mkdir /app

RUN apk update
RUN apk add postgresql git make
RUN go get -u github.com/jteeuwen/go-bindata/...

COPY ./bin/maestro-linux-amd64 /app/maestro
COPY ./config/local.yaml /app/config/local.yaml
COPY ./Makefile /app/Makefile
COPY ./scripts/drop.sql /app/scripts/drop.sql
COPY ./migrations /app/migrations

WORKDIR /app
RUN make assets

EXPOSE 8080

ENV MAESTRO_EXTENSIONS_PG_HOST "maestro-postgres"
ENV MAESTRO_EXTENSIONS_PG_PORT "5432"
ENV MAESTRO_EXTENSIONS_PG_USER "maestro"
ENV MAESTRO_EXTENSIONS_PG_PASS "pass"
ENV PGPASSWORD "pass"

CMD /app/maestro start
