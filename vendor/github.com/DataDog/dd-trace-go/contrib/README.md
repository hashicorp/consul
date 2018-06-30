# Libraries supported for tracing

All of these libraries are supported by our Application Performance Monitoring tool.

## Usage

1. Check if your library is supported (*i.e.* you find it in this directory).  
*ex:* if you're using the `net/http` package for your server, you see it's present in this directory.

2. In your app, replace your import by our traced version of the library.  
*ex:*
```go
import "net/http"
```
becomes
```go
import "github.com/DataDog/dd-trace-go/contrib/net/http"
```

3. Read through the `example_test.go` present in each folder of the libraries to understand how to trace your app.  
*ex:* for `net/http`, see [net/http/example_test.go](https://github.com/DataDog/dd-trace-go/blob/master/contrib/net/http/example_test.go)

## Contribution guidelines

### 1. Follow the package naming convention

If a library looks like this: `github.com/user/lib`, the contribution must looks like this `user/lib`.
In the case of the standard library, just use the path after `src`.
*E.g.* `src/database/sql` becomes `database/sql`.

### 2. Respect the original API

Keep the original names for exported functions, don't use the prefix or suffix `trace`.
*E.g.* prefer `Open` instead of `OpenTrace`.

Of course you can modify the number of arguments of a function if you need to pass the tracer for example.
