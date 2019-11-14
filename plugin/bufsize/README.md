# bufsize
## Name
*bufsize* - sizes EDNS0 buffer size to prevent IP fragmentation.

## Description
*bufsize* limits a requester's UDP payload size.
It prevents IP fragmentation so that to deal with DNS vulnerability.

## Syntax
```txt
bufsize [SIZE]
```

**[SIZE]** is an int value for setting the buffer size.
The default value is 512, and the value must be within 512 - 4096.
Only one argument is acceptable, and it covers both IPv4 and IPv6.

## Examples
Enable limiting the buffer size of outgoing query to the resolver (172.31.0.10):
```corefile
. {
    bufsize 512
    forward . 172.31.0.10
    log
}
```

Enable limiting the buffer size as an authoritative nameserver:
```corefile
. {
    bufsize 512
    file db.example.org
    log
}
```

## Considerations
- Setting 1232 bytes to bufsize may avoid fragmentation on the majority of networks in use today, but it depends on the MTU of the physical network links.
- For now, if a client does not use EDNS, this plugin adds OPT RR.
