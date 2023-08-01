# build environment
FROM golang:1.20-alpine as build
RUN mkdir -vp /app
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
    go build -o livemix.bin . && \
    cp -v ./livemix.bin /usr/bin/

# deploy environment
FROM alpine
COPY --from=build /usr/bin/livemix.bin /usr/bin/
ENTRYPOINT ["/usr/bin/livemix.bin"]
