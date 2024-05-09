# build environment
FROM golang:1.22-alpine as build
RUN apk add --update gcc musl-dev && \
    mkdir -vp /app
COPY ./go.mod /app/go.mod
COPY ./go.sum /app/go.sum
COPY ./api /app/api
COPY ./bin /app/bin
COPY ./common /app/common
COPY ./control /app/control
COPY ./db /app/db
COPY ./edge /app/edge
COPY ./forwarder /app/forwarder
COPY ./hls /app/hls
COPY ./tracker /app/tracker
COPY ./utils /app/utils
COPY ./vod /app/vod
COPY ./main.go /app/main.go
RUN cd /app && \
    CGO_ENABLED=1 go build -o livemix.bin . && \
    go build -o livemix-util.bin ./bin/util/... && \
    cp -v ./livemix.bin ./livemix-util.bin /usr/bin/

# deploy environment
FROM alpine
COPY --from=build /usr/bin/livemix.bin /usr/bin/
COPY --from=build /usr/bin/livemix-util.bin /usr/bin/
ENTRYPOINT ["/usr/bin/livemix.bin"]
