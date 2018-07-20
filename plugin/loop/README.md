# loop

## Name

*loop* - detect forwarding loops and halt the server.

## Description

The *loop* plugin will send a random query to ourselves and will then keep track of how many times
we see it. If we see it more than twice, we assume CoreDNS is looping and we halt the process.

The plugin will try to send the query for up to 30 seconds. This is done to give CoreDNS enough time
to start up. Once a query has been successfully sent *loop* disables itself to prevent a query of
death.

The query send is `<random number>.<random number>.zone` with type set to HINFO.

## Syntax

~~~ txt
loop
~~~

## Examples

Start a server on the default port and load the *loop* and *forward* plugins. The *forward* plugin
forwards to it self.

~~~ txt
. {
    loop
    forward . 127.0.0.1
}
~~~

After CoreDNS has started it stops the process while logging:

~~~ txt
plugin/loop: Seen "HINFO IN 5577006791947779410.8674665223082153551." more than twice, loop detected
~~~
