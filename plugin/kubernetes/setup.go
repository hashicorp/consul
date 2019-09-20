package kubernetes

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/parse"
	"github.com/coredns/coredns/plugin/pkg/upstream"

	"github.com/caddyserver/caddy"
	"github.com/miekg/dns"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Pull this in setting klog's output to stdout
	"k8s.io/klog"

	// Excluding azure because it is failing to compile
	// pull this in here, because we want it excluded if plugin.cfg doesn't have k8s
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// pull this in here, because we want it excluded if plugin.cfg doesn't have k8s
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// pull this in here, because we want it excluded if plugin.cfg doesn't have k8s
	_ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
	"k8s.io/client-go/tools/clientcmd"
)

var log = clog.NewWithPlugin("kubernetes")

func init() { plugin.Register("kubernetes", setup) }

func setup(c *caddy.Controller) error {
	klog.SetOutput(os.Stdout)

	k, err := kubernetesParse(c)
	if err != nil {
		return plugin.Error("kubernetes", err)
	}

	err = k.InitKubeCache()
	if err != nil {
		return plugin.Error("kubernetes", err)
	}

	k.RegisterKubeCache(c)

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		k.Next = next
		return k
	})

	// get locally bound addresses
	c.OnStartup(func() error {
		k.localIPs = boundIPs(c)
		return nil
	})

	return nil
}

// RegisterKubeCache registers KubeCache start and stop functions with Caddy
func (k *Kubernetes) RegisterKubeCache(c *caddy.Controller) {
	c.OnStartup(func() error {
		go k.APIConn.Run()

		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				if k.APIConn.HasSynced() {
					return nil
				}
			case <-timeout:
				return nil
			}
		}
	})

	c.OnShutdown(func() error {
		return k.APIConn.Stop()
	})
}

func kubernetesParse(c *caddy.Controller) (*Kubernetes, error) {
	var (
		k8s *Kubernetes
		err error
	)

	i := 0
	for c.Next() {
		if i > 0 {
			return nil, plugin.ErrOnce
		}
		i++

		k8s, err = ParseStanza(c)
		if err != nil {
			return k8s, err
		}
	}
	return k8s, nil
}

// ParseStanza parses a kubernetes stanza
func ParseStanza(c *caddy.Controller) (*Kubernetes, error) {

	k8s := New([]string{""})
	k8s.autoPathSearch = searchFromResolvConf()

	opts := dnsControlOpts{
		initEndpointsCache: true,
		ignoreEmptyService: false,
	}
	k8s.opts = opts

	zones := c.RemainingArgs()

	if len(zones) != 0 {
		k8s.Zones = zones
		for i := 0; i < len(k8s.Zones); i++ {
			k8s.Zones[i] = plugin.Host(k8s.Zones[i]).Normalize()
		}
	} else {
		k8s.Zones = make([]string, len(c.ServerBlockKeys))
		for i := 0; i < len(c.ServerBlockKeys); i++ {
			k8s.Zones[i] = plugin.Host(c.ServerBlockKeys[i]).Normalize()
		}
	}

	k8s.primaryZoneIndex = -1
	for i, z := range k8s.Zones {
		if dnsutil.IsReverse(z) > 0 {
			continue
		}
		k8s.primaryZoneIndex = i
		break
	}

	if k8s.primaryZoneIndex == -1 {
		return nil, errors.New("non-reverse zone name must be used")
	}

	k8s.Upstream = upstream.New()

	for c.NextBlock() {
		switch c.Val() {
		case "endpoint_pod_names":
			args := c.RemainingArgs()
			if len(args) > 0 {
				return nil, c.ArgErr()
			}
			k8s.endpointNameMode = true
			continue
		case "pods":
			args := c.RemainingArgs()
			if len(args) == 1 {
				switch args[0] {
				case podModeDisabled, podModeInsecure, podModeVerified:
					k8s.podMode = args[0]
				default:
					return nil, fmt.Errorf("wrong value for pods: %s,  must be one of: disabled, verified, insecure", args[0])
				}
				continue
			}
			return nil, c.ArgErr()
		case "namespaces":
			args := c.RemainingArgs()
			if len(args) > 0 {
				for _, a := range args {
					k8s.Namespaces[a] = struct{}{}
				}
				continue
			}
			return nil, c.ArgErr()
		case "endpoint":
			args := c.RemainingArgs()
			if len(args) > 0 {
				// Multiple endpoints are deprecated but still could be specified,
				// only the first one be used, though
				k8s.APIServerList = args
				if len(args) > 1 {
					log.Warningf("Multiple endpoints have been deprecated, only the first specified endpoint '%s' is used", args[0])
				}
				continue
			}
			return nil, c.ArgErr()
		case "tls": // cert key cacertfile
			args := c.RemainingArgs()
			if len(args) == 3 {
				k8s.APIClientCert, k8s.APIClientKey, k8s.APICertAuth = args[0], args[1], args[2]
				continue
			}
			return nil, c.ArgErr()
		case "resyncperiod":
			continue
		case "labels":
			args := c.RemainingArgs()
			if len(args) > 0 {
				labelSelectorString := strings.Join(args, " ")
				ls, err := meta.ParseToLabelSelector(labelSelectorString)
				if err != nil {
					return nil, fmt.Errorf("unable to parse label selector value: '%v': %v", labelSelectorString, err)
				}
				k8s.opts.labelSelector = ls
				continue
			}
			return nil, c.ArgErr()
		case "namespace_labels":
			args := c.RemainingArgs()
			if len(args) > 0 {
				namespaceLabelSelectorString := strings.Join(args, " ")
				nls, err := meta.ParseToLabelSelector(namespaceLabelSelectorString)
				if err != nil {
					return nil, fmt.Errorf("unable to parse namespace_label selector value: '%v': %v", namespaceLabelSelectorString, err)
				}
				k8s.opts.namespaceLabelSelector = nls
				continue
			}
			return nil, c.ArgErr()
		case "fallthrough":
			k8s.Fall.SetZonesFromArgs(c.RemainingArgs())
		case "upstream":
			// remove soon
			c.RemainingArgs() // eat remaining args
		case "ttl":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return nil, c.ArgErr()
			}
			t, err := strconv.Atoi(args[0])
			if err != nil {
				return nil, err
			}
			if t < 0 || t > 3600 {
				return nil, c.Errf("ttl must be in range [0, 3600]: %d", t)
			}
			k8s.ttl = uint32(t)
		case "transfer":
			tos, froms, err := parse.Transfer(c, false)
			if err != nil {
				return nil, err
			}
			if len(froms) != 0 {
				return nil, c.Errf("transfer from is not supported with this plugin")
			}
			k8s.TransferTo = tos
		case "noendpoints":
			if len(c.RemainingArgs()) != 0 {
				return nil, c.ArgErr()
			}
			k8s.opts.initEndpointsCache = false
		case "ignore":
			args := c.RemainingArgs()
			if len(args) > 0 {
				ignore := args[0]
				if ignore == "empty_service" {
					k8s.opts.ignoreEmptyService = true
					continue
				} else {
					return nil, fmt.Errorf("unable to parse ignore value: '%v'", ignore)
				}
			}
		case "kubeconfig":
			args := c.RemainingArgs()
			if len(args) == 2 {
				config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
					&clientcmd.ClientConfigLoadingRules{ExplicitPath: args[0]},
					&clientcmd.ConfigOverrides{CurrentContext: args[1]},
				)
				k8s.ClientConfig = config
				continue
			}
			return nil, c.ArgErr()
		default:
			return nil, c.Errf("unknown property '%s'", c.Val())
		}
	}

	if len(k8s.Namespaces) != 0 && k8s.opts.namespaceLabelSelector != nil {
		return nil, c.Errf("namespaces and namespace_labels cannot both be set")
	}

	return k8s, nil
}

func searchFromResolvConf() []string {
	rc, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	plugin.Zones(rc.Search).Normalize()
	return rc.Search
}
