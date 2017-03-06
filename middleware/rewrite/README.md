# rewrite

*rewrite* performs internal message rewriting. Rewrites are invisible to the client.
There are simple rewrites (fast) and complex rewrites (slower), but they're powerful enough to
accommodate most dynamic back-end applications.

## Syntax

~~~
rewrite FIELD FROM TO
~~~

* **FIELD** is (`type`, `class`, `name`, ...)
* **FROM** is the exact name of type to match
* **TO** is the destination name or type to rewrite to

When the FIELD is `type` and FROM is (`A`, `MX`, etc.), the type of the message will be rewritten;
e.g., to rewrite ANY queries to HINFO, use `rewrite type ANY HINFO`.

When the FIELD is `class` and FROM is (`IN`, `CH`, or `HS`) the class of the message will be
rewritten; e.g., to rewrite CH queries to IN use `rewrite class CH IN`.

When the FIELD is `name` the query name in the message is rewritten; this
needs to be a full match of the name, e.g., `rewrite name miek.nl example.org`.

When the FIELD is `edns0` an EDNS0 option can be appended to the request as described below.

If you specify multiple rules and an incoming query matches on multiple (simple) rules, only
the first rewrite is applied.

## EDNS0 Options

Using FIELD edns0, you can set, append, or replace specific EDNS0 options on the request.

* `replace` will modify any matching (what that means may vary based on EDNS0 type) option with the specified option
* `append` will add the option regardless of what options already exist
* `set` will modify a matching option or add one if none is found

Currently supported are `EDNS0_LOCAL` and `EDNS0_NSID`.

### `EDNS0_LOCAL`

This has two fields, code and data. A match is defined as having the same code. Data may be a string, or if
it starts with `0x` it will be treated as hex. Example:

~~~
rewrite edns0 local set 0xffee 0x61626364
~~~

rewrites the first local option with code 0xffee, setting the data to "abcd". Equivalent:

~~~
rewrite edns0 local set 0xffee abcd
~~~

### `EDNS0_NSID`

This has no fields; it will add an NSID option with an empty string for the NSID. If the option already exists
and the action is `replace` or `set`, then the NSID in the option will be set to the empty string.
