# Writing middleware

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
