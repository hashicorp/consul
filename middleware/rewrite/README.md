# rewrite

*rewrite* performs internal message rewriting. Rewrites are invisible to the client.
There are simple rewrites (fast) and complex rewrites (slower), but they're powerful enough to
accommodate most dynamic back-end applications.

## Syntax

~~~
rewrite FROM TO
~~~

* **FROM** is the exact name of type to match
* **TO** is the destination name or type to rewrite to

If from *and* to look like a DNS type (`A`, `MX`, etc.), the type of the message will be rewriten;
e.g., to rewrite ANY queries to HINFO, use `rewrite ANY HINFO`.

If from *and* to look like a DNS class (`IN`, `CH`, or `HS`) the class of the message will be
rewritten; e.g., to rewrite CH queries to IN use `rewrite CH IN`.

If it does not look like a type a name is assumed and the qname in the message is rewritten; this
needs to be a full match of the name, e.g., `rewrite miek.nl example.org`.

If you specify multiple rules and an incoming query matches on multiple (simple) rules, only
the first rewrite is applied.

> Everything below this line has not been implemented, yet.

~~~
rewrite [basename] {
    regexp pattern
    ext    extensions...
    if     a cond b
    status code
    to     destinations...
}
~~~

* basepath is the base path to match before rewriting with a regular expression. Default is /.
* regexp (shorthand: r) will match the path with the given regular expression pattern. Extremely high-load servers should avoid using regular expressions.
* extensions... is a space-separated list of file extensions to include or ignore. Prefix an extension with ! to exclude an extension. The forward slash / symbol matches paths without file extensions.
* if specifies a rewrite condition. Multiple ifs are AND-ed together. a and b are any string and may use request placeholders. cond is the condition, with possible values explained below.
* status will respond with the given status code instead of performing a rewrite. In other words, use either "status" or "to" in your rule, but not both. The code must be a number in the format 2xx or 4xx.
* destinations... is one or more space-separated paths to rewrite to, with support for request placeholders as well as numbered regular expression captures such as {1}, {2}, etc. Rewrite will check each destination in order and rewrite to the first destination that exists. Each one is checked as a file or, if it ends with /, as a directory. The last destination will act as the default if no other destination exists.
"if" Conditions

The if keyword is a powerful way to describe your rule. It takes the format a cond b, where the values a and b are separated by cond, a condition. The condition can be any of these:

~~~
is = a equals b
not = a does NOT equal b
has = a has b as a substring (b is a substring of a)
not_has = b is NOT a substring of a
starts_with = b is a prefix of a
ends_with = b is a suffix of a
match = a matches b, where b is a regular expression
not_match = a does NOT match b, where b is a regular expression
~~~

## Examples

When requests come in for /mobile, actually serve /mobile/index.
rewrite /mobile /mobile/index
If the file is not favicon.ico and it is not a valid file or directory, serve the maintenance page if present, or finally, rewrite to index.php.

~~~
rewrite {
    if {file} not favicon.ico
    to {path} {path}/ /maintenance.html /index.php
}
~~~

If user agent includes "mobile" and path is not a valid file/directory, rewrite to the mobile index page.

~~~
rewrite {
    if if {>User-agent} has mobile
    to {path} {path}/ /mobile/index.php
}
~~~

If the request path starts with /source, respond with HTTP 403 Forbidden.

~~~
rewrite {
    regexp ^/source
    status 403
}
~~~

Rewrite /app to /index with a query string. {1} is the matched group (.*).

~~~
rewrite /app {
    r  (.*)
    to /index?path={1}
}
~~~
