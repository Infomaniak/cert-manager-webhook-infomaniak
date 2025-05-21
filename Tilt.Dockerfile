FROM golang:1.24.3-alpine3.21 AS build

RUN go install github.com/go-delve/delve/cmd/dlv@v1.24

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

ADD https://raw.githubusercontent.com/tilt-dev/rerun-process-wrapper/master/restart.sh /restart.sh
ADD https://raw.githubusercontent.com/tilt-dev/rerun-process-wrapper/master/start.sh /start.sh
RUN chmod +x /start.sh && chmod +x /restart.sh && touch /process.txt && chmod 0777 /process.txt
COPY webhook /usr/local/bin/webhook
COPY --from=build /go/bin/dlv /usr/local/bin/dlv

ENTRYPOINT ["/start.sh", "webhook"]
