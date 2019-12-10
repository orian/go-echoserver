FROM golang:1.13.4 as builder
ENV GO111MODULE=on

WORKDIR /go/src/
COPY . /go/src/github.com/orian/go-echoserver/
RUN cd /go/src/github.com/orian/go-echoserver/ && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o echoserver .

FROM alpine:latest
WORKDIR /app
EXPOSE 80 8080

COPY --from=builder /go/src/github.com/orian/go-echoserver/echoserver /app/echoserver
COPY start-echo.sh /app/start-echo.sh

RUN apk add --no-cache bash ca-certificates && chmod +x /app/echoserver /app/start-echo.sh

# init then run up migration
CMD ["/app/start-echo.sh"]