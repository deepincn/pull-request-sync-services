FROM golang:alpine

WORKDIR /src/app

COPY . .

RUN apk add gcc
RUN apk add g++
RUN apk add git
RUN apk add git-review
RUN apk add openssh

RUN env GIN_MODE=release go build

CMD ["/src/app/pull-request-sync-services"]
