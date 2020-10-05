FROM golang
ADD . /go/src/edge-gw-trace-service
RUN go install edge-gw-trace-service
EXPOSE 8080