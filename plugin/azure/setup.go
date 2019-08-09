package azure

import (
	"context"
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"

	azuredns "github.com/Azure/azure-sdk-for-go/profiles/latest/dns/mgmt/dns"
	azurerest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/caddyserver/caddy"
)

var log = clog.NewWithPlugin("azure")

func init() {
	caddy.RegisterPlugin("azure", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	env, keys, fall, err := parse(c)
	if err != nil {
		return plugin.Error("azure", err)
	}
	ctx := context.Background()

	dnsClient := azuredns.NewRecordSetsClient(env.Values[auth.SubscriptionID])
	if dnsClient.Authorizer, err = env.GetAuthorizer(); err != nil {
		return plugin.Error("azure", err)
	}

	h, err := New(ctx, dnsClient, keys)
	if err != nil {
		return plugin.Error("azure", err)
	}
	h.Fall = fall
	if err := h.Run(ctx); err != nil {
		return plugin.Error("azure", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		h.Next = next
		return h
	})
	return nil
}

func parse(c *caddy.Controller) (auth.EnvironmentSettings, map[string][]string, fall.F, error) {
	resourceGroupMapping := map[string][]string{}
	resourceGroupSet := map[string]struct{}{}
	azureEnv := azurerest.PublicCloud
	env := auth.EnvironmentSettings{Values: map[string]string{}}

	var fall fall.F

	for c.Next() {
		args := c.RemainingArgs()

		for i := 0; i < len(args); i++ {
			parts := strings.SplitN(args[i], ":", 2)
			if len(parts) != 2 {
				return env, resourceGroupMapping, fall, c.Errf("invalid resource group/zone: %q", args[i])
			}
			resourceGroup, zoneName := parts[0], parts[1]
			if resourceGroup == "" || zoneName == "" {
				return env, resourceGroupMapping, fall, c.Errf("invalid resource group/zone: %q", args[i])
			}
			if _, ok := resourceGroupSet[args[i]]; ok {
				return env, resourceGroupMapping, fall, c.Errf("conflicting zone: %q", args[i])
			}

			resourceGroupSet[args[i]] = struct{}{}
			resourceGroupMapping[resourceGroup] = append(resourceGroupMapping[resourceGroup], zoneName)
		}
		for c.NextBlock() {
			switch c.Val() {
			case "subscription":
				if !c.NextArg() {
					return env, resourceGroupMapping, fall, c.ArgErr()
				}
				env.Values[auth.SubscriptionID] = c.Val()
			case "tenant":
				if !c.NextArg() {
					return env, resourceGroupMapping, fall, c.ArgErr()
				}
				env.Values[auth.TenantID] = c.Val()
			case "client":
				if !c.NextArg() {
					return env, resourceGroupMapping, fall, c.ArgErr()
				}
				env.Values[auth.ClientID] = c.Val()
			case "secret":
				if !c.NextArg() {
					return env, resourceGroupMapping, fall, c.ArgErr()
				}
				env.Values[auth.ClientSecret] = c.Val()
			case "environment":
				if !c.NextArg() {
					return env, resourceGroupMapping, fall, c.ArgErr()
				}
				env.Values[auth.ClientSecret] = c.Val()
				var err error
				if azureEnv, err = azurerest.EnvironmentFromName(c.Val()); err != nil {
					return env, resourceGroupMapping, fall, c.Errf("cannot set azure environment: %q", err.Error())
				}
			case "fallthrough":
				fall.SetZonesFromArgs(c.RemainingArgs())
			default:
				return env, resourceGroupMapping, fall, c.Errf("unknown property: %q", c.Val())
			}
		}
	}

	env.Values[auth.Resource] = azureEnv.ResourceManagerEndpoint
	env.Environment = azureEnv

	return env, resourceGroupMapping, fall, nil
}
