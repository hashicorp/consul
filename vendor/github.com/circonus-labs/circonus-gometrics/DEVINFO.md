# Setting up dev/test environment

Get go installed and environment configured

```sh

cd $GOPATH
mkdir -pv src/github.com/{hashicorp,armon,circonus-labs}

cd $GOPATH/src/github.com/hashicorp
git clone https://github.com/maier/consul.git

cd $GOPATH/src/github.com/armon
git clone https://github.com/maier/go-metrics.git

cd $GOPATH/src/github.com/circonus-labs
git clone https://github.com/maier/circonus-gometrics.git


cd $GOPATH/src/github.com/hashicorp/consul
make dev
```

In `$GOPATH/src/github.com/hashicorp/consul/bin` is the binary just created.

Create a consul configuration file somewhere (e.g. ~/testconfig.json) and add any applicable configuration settings. As an example:

```json
{
    "datacenter": "mcfl",
    "server": true,
    "log_level": "debug",
    "telemetry": {
        "statsd_address": "127.0.0.1:8125",
        "circonus_api_token": "...",
        "circonus_api_host": "..."
    }
}
```

StatsD was used as a check to see what metrics consul was sending and what metrics circonus was receiving. So, it can safely be elided.

Fill in appropriate cirocnus specific settings:

* circonus_api_token - required
* circonus_api_app - optional, default is circonus-gometrics
* circonus_api_host - optional, default is api.circonus.com (for dev stuff yon can use "http://..." to circumvent ssl)
* circonus_submission_url - optional
* circonus_submission_interval - optional, seconds, defaults to 10 seconds
* circonus_check_id - optional
* circonus_broker_id - optional (unless you want to use the public one, then add it)

The actual circonus-gometrics package has more configuraiton options, the above are exposed in the consul configuration.

CirconusMetrics.InstanceId is derived from consul's config.NodeName and config.Datacenter
CirconusMetrics.SearchTag is hardcoded as 'service:consul'

The defaults are taken for other options.

---

To run after creating the config:

`$GOPATH/src/github.com/hashicorp/consul/bin/consul agent -dev -config-file <config file>`

or, to add the ui (localhost:8500)

`$GOPATH/src/github.com/hashicorp/consul/bin/consul agent -dev -ui -config-file <config file>`

