# nsid

*nsid* add an identifier of this server to each reply.

This plugin implements RFC 5001 and adds an EDNS0 OPT resource record to replies that uniquely
identifies the server. This can be useful in anycast setups to see which server was responsible for
generating the reply and for debugging.

## Syntax

~~ txt
nsid [DATA]
~~

**DATA** is the string to use in the nsid record.

If **DATA** is not given, the host's name is used.

## Examples

Enable nsid:

~~ corefile
. {
    nsid
}
~~
