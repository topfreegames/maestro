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

RUN mkdir -p /app

RUN go install github.com/jteeuwen/go-bindata/...
COPY . /go/src/github.com/topfreegames/maestro

RUN cd /go/src/github.com/topfreegames/maestro && \
  make build/rooms-api && \
  make build/utils && \
  mv bin/rooms-api /app/rooms-api && \
  mv bin/utils /app/utils && \
  mv config /app/config && \
  mv kubeconfig.yaml /app/kubeconfig.yaml && \
  mv Makefile /app/Makefile

FROM debian:buster-slim

RUN apt update && apt install -y ca-certificates openssl

ARG CONFIG_FILE_PATH
ENV CONFIG_FILE_PATH_ENV=$CONFIG_FILE_PATH

WORKDIR /app

COPY --from=build-env /app/rooms-api /app/rooms-api
COPY --from=build-env /app/utils /app/utils
COPY --from=build-env /app/Makefile /app/Makefile
COPY --from=build-env /app/config /app/config
COPY --from=build-env /app/kubeconfig.yaml /app/kubeconfig.yaml

EXPOSE 8090

CMD ["sh", "-c", "/app/rooms-api --config-path=${CONFIG_FILE_PATH_ENV}"]
