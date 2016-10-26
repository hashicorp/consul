# Middleware

## Writing Middleware

From the Caddy docs:

> Oh yes, those pesky return values on ServeHTTP(). You read the documentation so you already know
> what they mean. But what does that imply for the behavior of your middleware?
>
> Basically, return a status code only if you did NOT write to the response body. If you DO write to
> the response body, return a status code of 0. Return an error value if your middleware encountered
> an error that you want logged. It is common to return an error status and an error value together,
> so that the error handler up the chain can write the correct error page.
>
> The returned status code is not logged directly; rather, it tells middleware higher up the chain
> what status code to use if/when the response body is written. Again, return a 0 status if you've
> already written a body!

In the DNS status codes are called rcodes and it's slightly harder to return the correct
answer in case of failure.

So CoreDNS treats:

* SERVFAIL (dns.RcodeServerFailure)
* REFUSED (dns.RecodeRefused)
* FORMERR (dns.RcodeFormatError)
* NOTIMP (dns.RcodeNotImplemented)

as special and will then assume nothing has written to the client. In all other cases it is assumes
something has been written to the client (by the middleware).

## Hooking it up

TODO(miek): text here on how to hook up middleware.

## Metrics

When exporting metrics the *Namespace* should be `middleware.Namespace` (="coredns"), and the
*Subsystem* should be the name of the middleware. The README.md for the middleware should then
also contain a *Metrics* section detailing the metrics.

## Documentation

Each middleware should have a README.md explaining what the middleware does and how it is
configured. The file should have the following layout:

* Title: use the middleware's name
* Subsection titled: "Syntax"
* Subsection titled: "Examples"

More sections are of course possible.

### Style

We use the Unix manual page style:

* The name of middleware in the running text should be italic: *middleware*.
* all CAPITAL: user supplied argument, in the running text references this use strong text: `**`:
  **EXAMPLE**.
* Optional text: in block quotes: `[optional]`.
* Use three dots to indicate multiple options are allowed: `arg...`.
* Item used literal: `literal`.
