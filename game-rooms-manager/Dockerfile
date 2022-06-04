FROM golang:1.17.3-alpine AS build-env

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . /build

EXPOSE 8080 8080

CMD go run main.go
