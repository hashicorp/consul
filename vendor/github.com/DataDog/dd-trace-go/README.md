[![CircleCI](https://circleci.com/gh/DataDog/dd-trace-go/tree/master.svg?style=svg)](https://circleci.com/gh/DataDog/dd-trace-go/tree/master)
[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/DataDog/dd-trace-go/opentracing)

Datadog APM client that implements an [OpenTracing](http://opentracing.io) Tracer.

## Initialization

To start using the Datadog Tracer with the OpenTracing API, you should first initialize the tracer with a proper `Configuration` object:

```go
import (
	// ddtrace namespace is suggested
	ddtrace "github.com/DataDog/dd-trace-go/opentracing"
	opentracing "github.com/opentracing/opentracing-go"
)

func main() {
	// create a Tracer configuration
	config := ddtrace.NewConfiguration()
	config.ServiceName = "api-intake"
	config.AgentHostname = "ddagent.consul.local"

	// initialize a Tracer and ensure a graceful shutdown
	// using the `closer.Close()`
	tracer, closer, err := ddtrace.NewTracer(config)
	if err != nil {
		// handle the configuration error
	}
	defer closer.Close()

	// set the Datadog tracer as a GlobalTracer
	opentracing.SetGlobalTracer(tracer)
	startWebServer()
}
```

Function `NewTracer(config)` returns an `io.Closer` instance that can be used to gracefully shutdown the `tracer`. It's recommended to always call the `closer.Close()`, otherwise internal buffers are not flushed and you may lose some traces.

## Usage

See [Opentracing documentation](https://github.com/opentracing/opentracing-go) for some usage patterns. Legacy documentation is available in [GoDoc format](https://godoc.org/github.com/DataDog/dd-trace-go/tracer).

## Contributing Quick Start

Requirements:

* Go 1.7 or later
* Docker
* Rake
* [gometalinter](https://github.com/alecthomas/gometalinter)

### Run the tests

Start the containers defined in `docker-compose.yml` so that integrations can be tested:

```
$ docker-compose up -d
$ ./wait-for-services.sh  # wait that all services are up and running
```

Fetch package's third-party dependencies (integrations and testing utilities):

```
$ rake init
```

This will only work if your working directory is in $GOPATH/src.

Now, you can run your tests via :

```
$ rake test:lint  # linting via gometalinter
$ rake test:all   # test the tracer and all integrations
$ rake test:race  # use the -race flag
```

## Further Reading

Automatically traced libraries and frameworks: https://godoc.org/github.com/DataDog/dd-trace-go/tracer#pkg-subdirectories
Sample code: https://godoc.org/github.com/DataDog/dd-trace-go/tracer#pkg-examples
