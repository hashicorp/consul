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

## Examples

Disable the ability to recover from crashes and show debug logging:

~~~ corefile
. {
    debug
}
~~~
