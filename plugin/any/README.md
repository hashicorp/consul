
# any

## Name

*any* - gives a minimal response to ANY queries.

## Description

*any* basically blocks ANY queries by responding to them with a short HINFO reply. See [RFC
8482](https://tools.ietf.org/html/rfc8482) for details.

## Syntax

~~~ txt
any
~~~

## Examples

~~~ corefile
example.org {
    whoami
    any
}
~~~

A `dig +nocmd ANY example.org +noall +answer` now returns:

~~~ txt
example.org.  8482	IN	HINFO	"ANY obsoleted" "See RFC 8482"
~~~

## Also See

[RFC 8482](https://tools.ietf.org/html/rfc8482).
