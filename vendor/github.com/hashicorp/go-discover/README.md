# Go Discover Nodes for Cloud Providers

`go-discover` is a Go (golang) library to discover ip addresses of nodes
in cloud environments based on meta information like tags provided by
the environment.

The following cloud providers have built-in support but additional providers
can be added to the `discover.Disoverers` map.

 * Amazon AWS
 * Google Cloud
 * Microsoft Azure
 
## Usage

First, install the library:

```
go get -u github.com/hashicorp/go-discover
```

All providers are configured with a "key=val key=val ..." format
strings where the values are URL encoded. The `discover.Discover`
function determines the provider through the `provider` key.

Example:

```
provider=aws region=eu-west-1 ...
```

### Amazon AWS

```
l := log.New(os.Stderr, "", log.LstdFlags)
args := "provider=aws region=eu-west-1 tag_key=consul tag_value=... access_key_id=... secret_access_key=..."
nodes, err := discover.Discover(args, l)
```

### Google Cloud

```
l := log.New(os.Stderr, "", log.LstdFlags)
args := "provider=gce project_name=... zone_pattern=eu-west-* tag_value=consul credentials_file=..."
nodes, err := discover.Discover(args, l)
```

### Microsoft Azure

```
l := log.New(os.Stderr, "", log.LstdFlags)
args := "provider=azure tag_name=consul tag_value=... tenant_id=... client_id=... subscription_id=... secret_access_key=..."
nodes, err := discover.Discover(args, l)
```

For complete API documentation, see [GoDoc](https://godoc.org/github.com/hashicorp/go-discover).

