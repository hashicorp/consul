# Go Discover Nodes for Cloud Providers [![Build Status](https://travis-ci.org/hashicorp/go-discover.svg?branch=master)](https://travis-ci.org/hashicorp/go-discover) [![GoDoc](https://godoc.org/github.com/hashicorp/go-discover?status.svg)](https://godoc.org/github.com/hashicorp/go-discover)


`go-discover` is a Go (golang) library and command line tool to discover
ip addresses of nodes in cloud environments based on meta information
like tags provided by the environment.

The configuration for the providers is provided as a list of `key=val
key=val ...` tuples where the values can be URL encoded. The provider is
determined through the `provider` key. Effectively, only spaces have to
be encoded with a `+` and on the command line you have to observe
quoting rules with your shell.

### Supported Providers

The following cloud providers have implementations in the go-discover/provider
sub packages. Additional providers can be added through the [Register](https://godoc.org/github.com/hashicorp/go-discover#Register)
function.

 * Amazon AWS [Config options](http://godoc.org/github.com/hashicorp/go-discover/aws)
 * Google Cloud [Config options](http://godoc.org/github.com/hashicorp/go-discover/gce)
 * Microsoft Azure [Config options](http://godoc.org/github.com/hashicorp/go-discover/azure)
 * SoftLayer [Config options](http://godoc.org/github.com/hashicorp/go-discover/softlayer)

### Config Example

```
# Amazon AWS
provider=aws region=eu-west-1 tag_key=consul tag_value=... access_key_id=... secret_access_key=...

# Google Cloud
provider=gce project_name=... zone_pattern=eu-west-* tag_value=consul credentials_file=...

# Microsoft Azure
provider=azure tag_name=consul tag_value=... tenant_id=... client_id=... subscription_id=... secret_access_key=...

# SoftLayer
provider=softlayer datacenter=dal06 tag_value=consul username=... api_key=...
```

## Command Line Tool Usage

Install the command line tool with:

```
go get -u github.com/hashicorp/go-discover/cmd/discover
```

Then run it with:

```
$ discover addrs provider=aws region=eu-west-1 ...
```

## Library Usage

Install the library with:

```
go get -u github.com/hashicorp/go-discover
```

Supported providers need to be registered by importing them similar
to database drivers for the `database/sql` package. 

Import the `go-discover` package and any provider package
you want to support.

```go
// support only AWS and GCE
import (
	discover "github.com/hashicorp/go-discover"

	_ "github.com/hashicorp/go-discover/provider/aws"
	_ "github.com/hashicorp/go-discover/provider/gce"
)

```

To import all known providers at once you can use the convenience 
package `all`.


```go
// support all providers supported by go-discover
import (
	discover "github.com/hashicorp/go-discover"

	_ "github.com/hashicorp/go-discover/provider/all"
)
```

Then call the `discover.Addrs` function with the arguments
for the provider you want to use:

```go
# use ioutil.Discard for no log output
l := log.New(os.Stderr, "", log.LstdFlags)
cfg := "provider=aws region=eu-west-1 ..."
addrs, err := discover.Addrs(cfg, l)
```

For complete API documentation, see
[GoDoc](https://godoc.org/github.com/hashicorp/go-discover). The configuration
for the supported providers is documented in the
[providers](https://godoc.org/github.com/hashicorp/go-discover/provider)
sub-package.
