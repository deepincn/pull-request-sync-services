FROM golang:alpine

WORKDIR /go/src/app

COPY . .

RUN go build

CMD ["pull-request-sync-services"]
