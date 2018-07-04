# debug

## Name

*debug* - disables the automatic recovery upon a crash so that you'll get a nice stack trace.

## Description

Normally CoreDNS will recover from panics, using *debug* inhibits this. The main use of *debug* is
to help testing. A side effect of using *debug* is that `log.Debug` and `log.Debugf` will be printed
to standard output.

Note that the *errors* plugin (if loaded) will also set a `recover` negating this setting.

## Syntax

~~~ txt
debug
~~~

Some plugin will debug log DNS messages. This is done in the following format:

~~~
debug: 000000 00 0a 01 00 00 01 00 00 00 00 00 01 07 65 78 61
debug: 000010 6d 70 6c 65 05 6c 6f 63 61 6c 00 00 01 00 01 00
debug: 000020 00 29 10 00 00 00 80 00 00 00
debug: 00002a
~~~

Using `text2pcap` (part of Wireshark) this can be converted back to binary, with the following
command line: `text2pcap -i 17 -u 53,53`. Where 17 is the protocol (UDP) and 53 are the ports. These
ports allow wireshark to detect these packets as DNS messages.

Each plugin can decide to dump messages to aid in debugging.

## Examples

Disable the ability to recover from crashes and show debug logging:

~~~ corefile
. {
    debug
}
~~~

## Also See

https://www.wireshark.org/docs/man-pages/text2pcap.html.
