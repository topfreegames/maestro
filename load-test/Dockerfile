FROM alpine

RUN apk add --no-cache go

RUN mkdir /app
ADD ./load-test /app
WORKDIR /app
EXPOSE 5050

CMD [ "./load-test", "server" ]
