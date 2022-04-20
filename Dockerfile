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

FROM golang:1.17.3-alpine AS build-env

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . /build

RUN mkdir -p /app

RUN apk add --update make

RUN cd /build && \
    make build-linux-x86_64 && \
    mv ./bin/maestro-linux-x86_64 /app/maestro && \
    mv internal/service/migrations /app/migrations && \
    mv ./config /app/config


FROM alpine

WORKDIR /app

COPY --from=build-env /app/maestro /app/maestro
COPY --from=build-env /app/migrations /app/migrations
COPY --from=build-env /app/config /app/config

EXPOSE 8080 8081

CMD /app/maestro
