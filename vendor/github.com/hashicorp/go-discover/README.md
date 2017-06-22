# Go Discover Nodes for Cloud Providers

`go-discover` is a Go (golang) library and command line tool to discover
ip addresses of nodes in cloud environments based on meta information
like tags provided by the environment.

The configuration for the providers is provided as a list of `key=val
key=val ...` tuples where the values can be URL encoded. The provider is
determined through the `provider` key. Effectively, only spaces have to
be encoded with a `+` and on the command line you have to observe
quoting rules with your shell.

### Example

```
# Amazon AWS
provider=aws region=eu-west-1 tag_key=consul tag_value=... access_key_id=... secret_access_key=...

# Google Cloud
provider=gce project_name=... zone_pattern=eu-west-* tag_value=consul credentials_file=...

# Microsoft Azure
provider=azure tag_name=consul tag_value=... tenant_id=... client_id=... subscription_id=... secret_access_key=...
```

### Supported Providers

The following cloud providers are supported but additional providers
can be added to the `discover.Disoverers` map.

 * Amazon AWS [Config options](http://godoc.org/github.com/hashicorp/go-discover/aws)
 * Google Cloud [Config options](http://godoc.org/github.com/hashicorp/go-discover/gce)
 * Microsoft Azure [Config options](http://godoc.org/github.com/hashicorp/go-discover/azure)

## Command Line Tool Usage

Install the command line tool with:

```
go get -u github.com/hashicorp/go-discover/cmd/discover
```

Then run it with:

```
$ discover provider=aws region=eu-west-1 ...
```

## Library Usage

Install the library with:

```
go get -u github.com/hashicorp/go-discover
```

Then call the `discover.Discover` function with the arguments
for the provider you want to use:


```go
# use ioutil.Discard for no log output
l := log.New(os.Stderr, "", log.LstdFlags)
args := "provider=aws region=eu-west-1 ..."
addrs, err := discover.Discover(args, l)
```

For complete API documentation, see [GoDoc](https://godoc.org/github.com/hashicorp/go-discover) and
the [supported providers](http://godoc.org/github.com/hashicorp/go-discover#pkg-subdirectories).

