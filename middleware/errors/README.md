# errors

`errors` allows you to set custom error pages and enable error logging.
By default, error responses (HTTP status >= 400) are not logged and the client receives a plaintext error message.
Using an error log, the text of each error will be recorded so you can determine what is going wrong without exposing those details to the clients. With error pages, you can present custom error messages and instruct your visitor with what to do.


## Syntax

~~~
errors [logfile]
~~~

* `logfile` is the path to the error log file to create (or append to), relative to the current working directory. It can also be stdout or stderr to write to the console, syslog to write to the system log (except on Windows), or visible to write the error (including full stack trace, if applicable) to the response. Writing errors to the response is NOT advised except in local debug situations. The default is stderr.
The above syntax will simply enable error reporting on the server. To specify custom error pages, open a block:

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

Maintain error log files automatically:

~~~
errors {
    log error.log {
        size 50 # Rotate after 50 MB
        age  30 # Keep rotated files for 30 days
        keep 5  # Keep at most 5 log files
    }
}
~~~
