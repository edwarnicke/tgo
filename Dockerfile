FROM golang:1.14.2-alpine3.11 as build
RUN apk add git
RUN GOPROXY=direct go get github.com/edwarnicke/tgo/cmd/tgo
ENV CGO_ENABLED="0"
WORKDIR /build/
COPY . .
RUN tgo build -o /bin ./...

FROM build as test
CMD tgo test ./...

FROM golang:1.14.2-alpine3.11 as runtime
COPY --from=build /bin/tgo /bin/tgo
CMD /bin/tgo

