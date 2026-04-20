# Service-Defaults Configuration Guide

This guide explains how to configure `service-defaults` config entries in Consul, specifically focusing on outlier detection (passive health checking).

## What is Service-Defaults?

`service-defaults` is a Consul config entry that defines default settings for a service, including:
- Protocol (http, http2, grpc, tcp)
- Upstream configuration (connection limits, health checks, etc.)
- Mesh gateway mode
- Transparent proxy settings
- And more...

## Configuration Methods

### 1. Via Consul CLI (HCL Format)

Create a file `web-defaults.hcl`:

```hcl
Kind = "service-defaults"
Name = "web"
Protocol = "http"

UpstreamConfig {
  Defaults {
    # Apply to all upstreams of this service
    PassiveHealthCheck {
      Interval = "30s"
      MaxFailures = 5
      EnforcingConsecutive5xx = 100
      MaxEjectionPercent = 10
      BaseEjectionTime = "30s"
    }
  }
  
  Overrides = [
    {
      # Override for specific upstream
      Name = "database"
      PassiveHealthCheck {
        Interval = "10s"
        MaxFailures = 3
        EnforcingConsecutive5xx = 100
        MaxEjectionPercent = 50
        BaseEjectionTime = "60s"
      }
    }
  ]
}
```

Apply it:
```bash
consul config write web-defaults.hcl
```

### 2. Via Consul API (JSON Format)

```bash
curl -X PUT http://localhost:8500/v1/config \
  -H "Content-Type: application/json" \
  -d '{
  "Kind": "service-defaults",
  "Name": "web",
  "Protocol": "http",
  "UpstreamConfig": {
    "Defaults": {
      "PassiveHealthCheck": {
        "Interval": "30s",
        "MaxFailures": 5,
        "EnforcingConsecutive5xx": 100,
        "MaxEjectionPercent": 10,
        "BaseEjectionTime": "30s"
      }
    }
  }
}'
```

### 3. Via Service Registration (Inline Config)

In your service registration file:

```hcl
services {
  name = "web"
  port = 8080
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "database"
            local_bind_port = 5432
            config {
              passive_health_check {
                interval = "22s"
                max_failures = 4
                enforcing_consecutive_5xx = 99
                max_ejection_percent = 50
                base_ejection_time = "60s"
              }
            }
          }
        ]
      }
    }
  }
}
```

Register it:
```bash
consul services register web-service.hcl
```

### 4. Via Consul Go API

```go
package main

import (
    "github.com/hashicorp/consul/api"
)

func main() {
    client, _ := api.NewClient(api.DefaultConfig())
    
    interval := 30 * time.Second
    maxFailures := uint32(5)
    enforcing := uint32(100)
    maxEjection := uint32(10)
    baseEjection := 30 * time.Second
    
    entry := &api.ServiceConfigEntry{
        Kind:     api.ServiceDefaults,
        Name:     "web",
        Protocol: "http",
        UpstreamConfig: &api.UpstreamConfiguration{
            Defaults: &api.UpstreamConfig{
                PassiveHealthCheck: &api.PassiveHealthCheck{
                    Interval:                interval,
                    MaxFailures:             maxFailures,
                    EnforcingConsecutive5xx: &enforcing,
                    MaxEjectionPercent:      &maxEjection,
                    BaseEjectionTime:        &baseEjection,
                },
            },
        },
    }
    
    _, _, err := client.ConfigEntries().Set(entry, nil)
    if err != nil {
        panic(err)
    }
}
```

## PassiveHealthCheck Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `Interval` | duration | Time between health check analysis sweeps | - |
| `MaxFailures` | uint32 | Consecutive failures before ejection | - |
| `EnforcingConsecutive5xx` | uint32 | % chance of ejection (0-100) | 100 |
| `MaxEjectionPercent` | uint32 | Max % of cluster that can be ejected | 10 |
| `BaseEjectionTime` | duration | Base ejection duration (multiplied by ejection count) | 30s |

## Configuration Hierarchy

Consul applies outlier detection configuration in this order (highest to lowest priority):

1. **Per-upstream inline config** (in service registration)
2. **Service-defaults overrides** (per-upstream in UpstreamConfig.Overrides)
3. **Service-defaults defaults** (UpstreamConfig.Defaults)
4. **Wildcard defaults** (service-defaults with Name = "*")
5. **Envoy defaults** (if no config specified)

## Real-World Examples

### Example 1: Basic Outlier Detection

```hcl
Kind = "service-defaults"
Name = "api"
Protocol = "http"

UpstreamConfig {
  Defaults {
    PassiveHealthCheck {
      Interval = "10s"
      MaxFailures = 3
    }
  }
}
```

This enables outlier detection with:
- Check every 10 seconds
- Eject after 3 consecutive failures
- Use Envoy defaults for other parameters

### Example 2: Aggressive Ejection for Critical Services

```hcl
Kind = "service-defaults"
Name = "payment-service"
Protocol = "http"

UpstreamConfig {
  Defaults {
    PassiveHealthCheck {
      Interval = "5s"
      MaxFailures = 2
      EnforcingConsecutive5xx = 100
      MaxEjectionPercent = 50
      BaseEjectionTime = "120s"
    }
  }
}
```

This configuration:
- Checks every 5 seconds
- Ejects after only 2 failures
- Always enforces ejection (100%)
- Can eject up to 50% of instances
- Keeps instances ejected for at least 2 minutes

### Example 3: Per-Upstream Overrides

```hcl
Kind = "service-defaults"
Name = "frontend"
Protocol = "http"

UpstreamConfig {
  # Default for all upstreams
  Defaults {
    PassiveHealthCheck {
      Interval = "30s"
      MaxFailures = 5
    }
  }
  
  # Stricter settings for critical database
  Overrides = [
    {
      Name = "postgres"
      PassiveHealthCheck {
        Interval = "10s"
        MaxFailures = 2
        MaxEjectionPercent = 30
      }
    },
    {
      Name = "redis"
      PassiveHealthCheck {
        Interval = "5s"
        MaxFailures = 3
      }
    }
  ]
}
```

## Verification

### 1. Check Config Entry

```bash
consul config read -kind service-defaults -name web
```

### 2. Verify in Envoy Config

```bash
# Get cluster configuration
curl http://localhost:19000/config_dump | jq '.configs[1].dynamic_active_clusters[] | select(.cluster.name=="db") | .cluster.outlier_detection'
```

Expected output:
```json
{
  "interval": "22s",
  "consecutive_5xx": 4,
  "enforcing_consecutive_5xx": 99,
  "max_ejection_percent": 50,
  "base_ejection_time": "60s"
}
```

### 3. Monitor Ejections

```bash
# Check Envoy stats for outlier detection
curl http://localhost:19000/stats | grep outlier_detection

# Example output:
# cluster.db.outlier_detection.ejections_active: 0
# cluster.db.outlier_detection.ejections_consecutive_5xx: 2
# cluster.db.outlier_detection.ejections_total: 2
```

## Common Patterns

### Pattern 1: Wildcard Defaults

Apply to all services:

```hcl
Kind = "service-defaults"
Name = "*"

UpstreamConfig {
  Defaults {
    PassiveHealthCheck {
      Interval = "30s"
      MaxFailures = 5
    }
  }
}
```

### Pattern 2: Disable Outlier Detection

Set all values to 0 or very high:

```hcl
Kind = "service-defaults"
Name = "legacy-service"

UpstreamConfig {
  Defaults {
    PassiveHealthCheck {
      MaxFailures = 999999
      EnforcingConsecutive5xx = 0
    }
  }
}
```

### Pattern 3: Gradual Rollout

Start with low enforcement, increase gradually:

```hcl
# Week 1: 25% enforcement
PassiveHealthCheck {
  EnforcingConsecutive5xx = 25
}

# Week 2: 50% enforcement
PassiveHealthCheck {
  EnforcingConsecutive5xx = 50
}

# Week 3: 100% enforcement
PassiveHealthCheck {
  EnforcingConsecutive5xx = 100
}
```

## Troubleshooting

### Issue: Outlier detection not working

**Check:**
1. Is EDS being used? (Hostname-based services don't support outlier detection)
2. Is the config entry applied? (`consul config read`)
3. Is Envoy receiving the config? (Check `/config_dump`)
4. Are there enough instances? (Need multiple endpoints to eject)

### Issue: Too many ejections

**Solution:** Increase `MaxEjectionPercent` or `MaxFailures`:

```hcl
PassiveHealthCheck {
  MaxFailures = 10
  MaxEjectionPercent = 30
}
```

### Issue: Instances not recovering

**Solution:** Decrease `BaseEjectionTime`:

```hcl
PassiveHealthCheck {
  BaseEjectionTime = "10s"
}
```

## Best Practices

1. **Start conservative**: Begin with high `MaxFailures` and low `MaxEjectionPercent`
2. **Monitor metrics**: Watch ejection stats before tightening thresholds
3. **Use overrides**: Apply stricter settings only to critical upstreams
4. **Test in staging**: Validate configuration before production
5. **Document decisions**: Record why specific thresholds were chosen
6. **Consider traffic patterns**: Adjust `Interval` based on request volume

## Related Documentation

- [Envoy Outlier Detection](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/outlier)
- [Consul Service Mesh](https://developer.hashicorp.com/consul/docs/connect)
- [Config Entries](https://developer.hashicorp.com/consul/docs/connect/config-entries)