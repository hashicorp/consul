package tfgen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/hashicorp/consul/consul-topology/topology"
	"github.com/hashicorp/consul/consul-topology/util"
)

func (g *Generator) getCoreDNSContainer(
	net *topology.Network,
	ipAddress string,
	hashes []string,
) Resource {
	var env []string
	for i, hv := range hashes {
		env = append(env, fmt.Sprintf("HASH_FILE_%d_VALUE=%s", i, hv))
	}
	coredns := struct {
		Name              string
		DockerNetworkName string
		IPAddress         string
		HashValues        string
		Env               []string
	}{
		Name:              net.Name,
		DockerNetworkName: net.DockerName,
		IPAddress:         ipAddress,
		Env:               env,
	}
	return Eval(tfCorednsT, &coredns)
}

func (g *Generator) writeCoreDNSFiles(net *topology.Network, dnsIPAddress string) (bool, []string, error) {
	if net.IsPublic() {
		return false, nil, fmt.Errorf("coredns only runs on local networks")
	}

	rootdir := filepath.Join(g.workdir, "terraform", "coredns-config-"+net.Name)
	if err := os.MkdirAll(rootdir, 0755); err != nil {
		return false, nil, err
	}

	for _, cluster := range g.topology.Clusters {
		if cluster.NetworkName != net.Name {
			continue
		}
		var addrs []string
		for _, node := range cluster.SortedNodes() {
			if node.Kind != topology.NodeKindServer || node.Disabled {
				continue
			}
			addr := node.AddressByNetwork(net.Name)
			if addr.IPAddress != "" {
				addrs = append(addrs, addr.IPAddress)
			}
		}

		var (
			clusterDNSName = cluster.Name + "-consulcluster.lan"
		)

		corefilePath := filepath.Join(rootdir, "Corefile")
		zonefilePath := filepath.Join(rootdir, "servers")

		_, err := UpdateFileIfDifferent(
			g.logger,
			generateCoreDNSConfigFile(
				clusterDNSName,
				addrs,
			),
			corefilePath,
			0644,
		)
		if err != nil {
			return false, nil, fmt.Errorf("error writing %q: %w", corefilePath, err)
		}
		corefileHash, err := util.HashFile(corefilePath)
		if err != nil {
			return false, nil, fmt.Errorf("error hashing %q: %w", corefilePath, err)
		}

		_, err = UpdateFileIfDifferent(
			g.logger,
			generateCoreDNSZoneFile(
				dnsIPAddress,
				clusterDNSName,
				addrs,
			),
			zonefilePath,
			0644,
		)
		if err != nil {
			return false, nil, fmt.Errorf("error writing %q: %w", zonefilePath, err)
		}
		zonefileHash, err := util.HashFile(zonefilePath)
		if err != nil {
			return false, nil, fmt.Errorf("error hashing %q: %w", zonefilePath, err)
		}

		return true, []string{corefileHash, zonefileHash}, nil
	}

	return false, nil, nil
}

func generateCoreDNSConfigFile(
	clusterDNSName string,
	addrs []string,
) []byte {
	serverPart := ""
	if len(addrs) > 0 {
		var servers []string
		for _, addr := range addrs {
			servers = append(servers, addr+":8600")
		}
		serverPart = fmt.Sprintf(`
consul:53 {
  forward . %s
  log
  errors
  whoami
}
`, strings.Join(servers, " "))
	}

	return []byte(fmt.Sprintf(`
%[1]s:53 {
  file /config/servers %[1]s
  log
  errors
  whoami
}

%[2]s

.:53 {
  forward . 8.8.8.8:53
  log
  errors
  whoami
}
`, clusterDNSName, serverPart))
}

func generateCoreDNSZoneFile(
	dnsIPAddress string,
	clusterDNSName string,
	addrs []string,
) []byte {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf(`
$TTL 60
$ORIGIN %[1]s.
@                   IN	SOA ns.%[1]s. webmaster.%[1]s. (
          2017042745 ; serial
          7200       ; refresh (2 hours)				
          3600       ; retry (1 hour)			
          1209600    ; expire (2 weeks)				
          3600       ; minimum (1 hour)				
          )
@  IN NS ns.%[1]s. ; Name server
ns IN A  %[2]s     ; self
`, clusterDNSName, dnsIPAddress))

	for _, addr := range addrs {
		buf.WriteString(fmt.Sprintf(`
server IN A %s ; Consul server
`, addr))
	}

	return buf.Bytes()
}

var tfCorednsT = template.Must(template.ParseFS(content, "templates/container-coredns.tf.tmpl"))
