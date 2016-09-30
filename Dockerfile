FROM golang:1.6

# Install dependencies.
RUN apt-get update && apt-get install -y ebtables

RUN go get -d -v golang.org/x/sys/unix

COPY . /go/src/github.com/Azure/Aqua
WORKDIR /go/src/github.com/Azure/Aqua

RUN go install

CMD ["/go/bin/Aqua"]
