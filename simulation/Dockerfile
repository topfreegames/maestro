FROM golang:1.23-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o room .

FROM alpine:edge

WORKDIR /app
COPY --from=build /app/room .

RUN apk --no-cache add ca-certificates tzdata

ENTRYPOINT ["/app/room"]
