# Dnstap

## Syntax

`dnstap SOCKET [full]`

* **SOCKET** is the socket path supplied to the dnstap command line tool.
* `full` to include the wire-format dns message.

## Dnstap command line tool

```sh
go get github.com/dnstap/golang-dnstap
cd $GOPATH/src/github.com/dnstap/golang-dnstap/dnstap
go build
./dnstap -u /tmp/dnstap.sock
./dnstap -u /tmp/dnstap.sock -y
```

There is a buffer, expect at least 13 requests before the server sends its dnstap messages to the socket.
