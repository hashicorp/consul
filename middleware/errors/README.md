# errors

*errors* enables error logging.
TODO: what are errors.

## Syntax

~~~
errors [LOGFILE]
~~~

* **LOGFILE** is the path to the error log file to create (or append to), relative to the current
  working directory. It can also be `stdout` or `stderr` to write to the console, syslog to write to the
  system log (except on Windows), or visible to write the error (including full stack trace, if
  applicable) to the response. Writing errors to the response is NOT advised except in local debug
  situations. The default is stderr. The above syntax will simply enable error reporting on the
  server. To specify custom error pages, open a block:

~~~
errors {
    what where
}
~~~

* `what` can only be `log`.
* `where` is the path to the log file (as described above) and you can enable rotation to manage the log files.

## Examples

Log errors into a file in the parent directory:

~~~
errors ../error.log
~~~

Make errors visible to the client (for debugging only):

~~~
errors visible
~~~
