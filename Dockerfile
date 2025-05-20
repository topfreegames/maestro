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

FROM golang:1.23-alpine AS build-env

# TARGETARCH and TARGETOS are automatically set by buildx
ARG TARGETARCH
ARG TARGETOS

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . /build

RUN mkdir -p /app

# Instead of 'make build', compile directly for the target platform
# Output the binary directly to /app/maestro
RUN echo "Building for $TARGETOS/$TARGETARCH" && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix cgo -o /app/maestro ./ && \
    mv /build/internal/service/migrations /app/migrations && \
    mv /build/config /app/config


FROM alpine

WORKDIR /app

COPY --from=build-env /app/maestro /app/maestro
COPY --from=build-env /app/migrations /app/migrations
COPY --from=build-env /app/config /app/config

EXPOSE 8080 8081

CMD ["/app/maestro"]
