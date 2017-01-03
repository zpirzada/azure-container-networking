FROM golang:latest

# Install dependencies.
RUN apt-get update && apt-get install -y ebtables

RUN go get -d -v golang.org/x/sys/unix

COPY . /go/src/github.com/Azure/azure-container-networking
WORKDIR /go/src/github.com/Azure/azure-container-networking

RUN make azure-cnm-plugin
RUN make azure-cni-plugin
