FROM golang
ADD . /go/src/github.com/armPelionEdge/edge-gw-trace-service
RUN go install github.com/armPelionEdge/edge-gw-trace-service
EXPOSE 8080
