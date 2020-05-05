FROM golang:1.14.2-alpine3.11 as build
RUN apk add git
RUN GOPROXY=direct go get github.com/edwarnicke/tgo/cmd/tgo
WORKDIR /build/
COPY . .
RUN tgo build ./...

