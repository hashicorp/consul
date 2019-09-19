# errors

## Name

*errors* - enables error logging.

## Description

Any errors encountered during the query processing will be printed to standard output. The errors of particular type can be consolidated and printed once per some period of time.

This plugin can only be used once per Server Block.

## Syntax

The basic syntax is:

~~~
errors
~~~

Extra knobs are available with an expanded syntax:

~~~
errors {
	consolidate DURATION REGEXP
}
~~~

Option `consolidate` allows collecting several error messages matching the regular expression **REGEXP** during **DURATION**. After the **DURATION** since receiving the first such message, the consolidated message will be printed to standard output, e.g.

~~~
2 errors like '^read udp .* i/o timeout$' occurred in last 30s
~~~

Multiple `consolidate` options with different **DURATION** and **REGEXP** are allowed. In case if some error message corresponds to several defined regular expressions the message will be associated with the first appropriate **REGEXP**.

For better performance, it's recommended to use the `^` or `$` metacharacters in regular expression when filtering error messages by prefix or suffix, e.g. `^failed to .*`, or `.* timeout$`.

## Examples

Use the *whoami* to respond to queries in the example.org domain and Log errors to standard output.

~~~ corefile
example.org {
    whoami
    errors
}
~~~

Use the *forward* to resolve queries via 8.8.8.8 and print consolidated error messages for errors with suffix " i/o timeout" or with prefix "Failed to ".

~~~ corefile
. {
    forward . 8.8.8.8
    errors {
        consolidate 5m ".* i/o timeout$"
        consolidate 30s "^Failed to .+"
    }
}
~~~
