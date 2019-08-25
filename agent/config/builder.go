package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-sockaddr/template"
	"golang.org/x/time/rate"
)

// Builder constructs a valid runtime configuration from multiple
// configuration sources.
//
// To build the runtime configuration first call Build() which merges
// the sources in a pre-defined order, converts the data types and
// structures into their final form and performs the syntactic
// validation.
//
// The sources are merged in the following order:
//
//  * default configuration
//  * config files in alphabetical order
//  * command line arguments
//
// The config sources are merged sequentially and later values
// overwrite previously set values. Slice values are merged by
// concatenating the two slices. Map values are merged by over-
// laying the later maps on top of earlier ones.
//
// Then call Validate() to perform the semantic validation to ensure
// that the configuration is ready to be used.
//
// Splitting the construction into two phases greatly simplifies testing
// since not all pre-conditions have to be satisfied when performing
// syntactical tests.
type Builder struct {
	// Flags contains the parsed command line arguments.
	Flags Flags

	// Head, Sources, and Tail are used to manage the order of the
	// config sources, as described in the comments above.
	Head    []Source
	Sources []Source
	Tail    []Source

	// Warnings contains the warnings encountered when
	// parsing the configuration.
	Warnings []string

	// Hostname returns the hostname of the machine. If nil, os.Hostname
	// is called.
	Hostname func() (string, error)

	// GetPrivateIPv4 and GetPublicIPv6 return suitable default addresses
	// for cases when the user doesn't supply them.
	GetPrivateIPv4 func() ([]*net.IPAddr, error)
	GetPublicIPv6  func() ([]*net.IPAddr, error)

	// err contains the first error that occurred during
	// building the runtime configuration.
	err error
}

// NewBuilder returns a new configuration builder based on the given command
// line flags.
func NewBuilder(flags Flags) (*Builder, error) {
	// We expect all flags to be parsed and flags.Args to be empty.
	// Therefore, we bail if we find unparsed args.
	if len(flags.Args) > 0 {
		return nil, fmt.Errorf("config: Unknown extra arguments: %v", flags.Args)
	}

	newSource := func(name string, v interface{}) Source {
		b, err := json.MarshalIndent(v, "", "    ")
		if err != nil {
			panic(err)
		}
		return Source{Name: name, Format: "json", Data: string(b)}
	}

	b := &Builder{
		Flags: flags,
		Head:  []Source{DefaultSource()},
	}

	if b.boolVal(b.Flags.DevMode) {
		b.Head = append(b.Head, DevSource())
	}

	// Since the merge logic is to overwrite all fields with later
	// values except slices which are merged by appending later values
	// we need to merge all slice values defined in flags before we
	// merge the config files since the flag values for slices are
	// otherwise appended instead of prepended.
	slices, values := b.splitSlicesAndValues(b.Flags.Config)
	b.Head = append(b.Head, newSource("flags.slices", slices))
	for _, path := range b.Flags.ConfigFiles {
		sources, err := b.ReadPath(path)
		if err != nil {
			return nil, err
		}
		b.Sources = append(b.Sources, sources...)
	}
	b.Tail = append(b.Tail, newSource("flags.values", values))
	for i, s := range b.Flags.HCL {
		b.Tail = append(b.Tail, Source{
			Name:   fmt.Sprintf("flags-%d.hcl", i),
			Format: "hcl",
			Data:   s,
		})
	}
	b.Tail = append(b.Tail, NonUserSource(), DefaultConsulSource(), DefaultEnterpriseSource(), DefaultVersionSource())
	if b.boolVal(b.Flags.DevMode) {
		b.Tail = append(b.Tail, DevConsulSource())
	}
	return b, nil
}

// ReadPath reads a single config file or all files in a directory (but
// not its sub-directories) and appends them to the list of config
// sources.
func (b *Builder) ReadPath(path string) ([]Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: Open failed on %s. %s", path, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("config: Stat failed on %s. %s", path, err)
	}

	if !fi.IsDir() {
		src, err := b.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return []Source{src}, nil
	}

	fis, err := f.Readdir(-1)
	if err != nil {
		return nil, fmt.Errorf("config: Readdir failed on %s. %s", path, err)
	}

	// sort files by name
	sort.Sort(byName(fis))

	var sources []Source
	for _, fi := range fis {
		fp := filepath.Join(path, fi.Name())
		// check for a symlink and resolve the path
		if fi.Mode()&os.ModeSymlink > 0 {
			var err error
			fp, err = filepath.EvalSymlinks(fp)
			if err != nil {
				return nil, err
			}
			fi, err = os.Stat(fp)
			if err != nil {
				return nil, err
			}
		}
		// do not recurse into sub dirs
		if fi.IsDir() {
			continue
		}

		if b.shouldParseFile(fp) {
			src, err := b.ReadFile(fp)
			if err != nil {
				return nil, err
			}
			sources = append(sources, src)
		}
	}
	return sources, nil
}

// ReadFile parses a JSON or HCL config file and appends it to the list of
// config sources.
func (b *Builder) ReadFile(path string) (Source, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return Source{}, fmt.Errorf("config: ReadFile failed on %s: %s", path, err)
	}
	return Source{Name: path, Data: string(data)}, nil
}

// shouldParse file determines whether the file to be read is of a supported extension
func (b *Builder) shouldParseFile(path string) bool {
	configFormat := b.stringVal(b.Flags.ConfigFormat)
	srcFormat := FormatFrom(path)

	// If config-format is not set, only read files with supported extensions
	if configFormat == "" && srcFormat != "hcl" && srcFormat != "json" {
		return false
	}
	return true
}

type byName []os.FileInfo

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].Name() < a[j].Name() }

func (b *Builder) BuildAndValidate() (RuntimeConfig, error) {
	rt, err := b.Build()
	if err != nil {
		return RuntimeConfig{}, err
	}
	if err := b.Validate(rt); err != nil {
		return RuntimeConfig{}, err
	}
	return rt, nil
}

// Build constructs the runtime configuration from the config sources
// and the command line flags. The config sources are processed in the
// order they were added with the flags being processed last to give
// precedence over the other sources. If the error is nil then
// warnings can still contain deprecation or format warnings that should
// be presented to the user.
func (b *Builder) Build() (rt RuntimeConfig, err error) {
	b.err = nil
	b.Warnings = nil

	// ----------------------------------------------------------------
	// merge config sources as follows
	//
	configFormat := b.stringVal(b.Flags.ConfigFormat)
	if configFormat != "" && configFormat != "json" && configFormat != "hcl" {
		return RuntimeConfig{}, fmt.Errorf("config: -config-format must be either 'hcl' or 'json'")
	}

	// build the list of config sources
	var srcs []Source
	srcs = append(srcs, b.Head...)
	for _, src := range b.Sources {
		// skip file if it should not be parsed
		if !b.shouldParseFile(src.Name) {
			continue
		}

		// if config-format is set, files of any extension will be interpreted in that format
		src.Format = FormatFrom(src.Name)
		if configFormat != "" {
			src.Format = configFormat
		}
		srcs = append(srcs, src)
	}
	srcs = append(srcs, b.Tail...)

	// parse the config sources into a configuration
	var c Config
	for _, s := range srcs {
		if s.Name == "" || s.Data == "" {
			continue
		}
		c2, err := Parse(s.Data, s.Format)
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("Error parsing %s: %s", s.Name, err)
		}

		// if we have a single 'check' or 'service' we need to add them to the
		// list of checks and services first since we cannot merge them
		// generically and later values would clobber earlier ones.
		if c2.Check != nil {
			c2.Checks = append(c2.Checks, *c2.Check)
			c2.Check = nil
		}
		if c2.Service != nil {
			c2.Services = append(c2.Services, *c2.Service)
			c2.Service = nil
		}

		c = Merge(c, c2)
	}

	// ----------------------------------------------------------------
	// process/merge some complex values
	//

	var dnsServiceTTL = map[string]time.Duration{}
	for k, v := range c.DNS.ServiceTTL {
		dnsServiceTTL[k] = b.durationVal(fmt.Sprintf("dns_config.service_ttl[%q]", k), &v)
	}

	soa := RuntimeSOAConfig{Refresh: 3600, Retry: 600, Expire: 86400, Minttl: 0}
	if c.DNS.SOA != nil {
		if c.DNS.SOA.Expire != nil {
			soa.Expire = *c.DNS.SOA.Expire
		}
		if c.DNS.SOA.Minttl != nil {
			soa.Minttl = *c.DNS.SOA.Minttl
		}
		if c.DNS.SOA.Refresh != nil {
			soa.Refresh = *c.DNS.SOA.Refresh
		}
		if c.DNS.SOA.Retry != nil {
			soa.Retry = *c.DNS.SOA.Retry
		}
	}

	leaveOnTerm := !b.boolVal(c.ServerMode)
	if c.LeaveOnTerm != nil {
		leaveOnTerm = b.boolVal(c.LeaveOnTerm)
	}

	skipLeaveOnInt := b.boolVal(c.ServerMode)
	if c.SkipLeaveOnInt != nil {
		skipLeaveOnInt = b.boolVal(c.SkipLeaveOnInt)
	}

	// ----------------------------------------------------------------
	// checks and services
	//

	var checks []*structs.CheckDefinition
	if c.Check != nil {
		checks = append(checks, b.checkVal(c.Check))
	}
	for _, check := range c.Checks {
		checks = append(checks, b.checkVal(&check))
	}

	var services []*structs.ServiceDefinition
	for _, service := range c.Services {
		services = append(services, b.serviceVal(&service))
	}
	if c.Service != nil {
		services = append(services, b.serviceVal(c.Service))
	}

	// ----------------------------------------------------------------
	// addresses
	//

	// determine port values and replace values <= 0 and > 65535 with -1
	dnsPort := b.portVal("ports.dns", c.Ports.DNS)
	httpPort := b.portVal("ports.http", c.Ports.HTTP)
	httpsPort := b.portVal("ports.https", c.Ports.HTTPS)
	serverPort := b.portVal("ports.server", c.Ports.Server)
	grpcPort := b.portVal("ports.grpc", c.Ports.GRPC)
	serfPortLAN := b.portVal("ports.serf_lan", c.Ports.SerfLAN)
	serfPortWAN := b.portVal("ports.serf_wan", c.Ports.SerfWAN)
	proxyMinPort := b.portVal("ports.proxy_min_port", c.Ports.ProxyMinPort)
	proxyMaxPort := b.portVal("ports.proxy_max_port", c.Ports.ProxyMaxPort)
	sidecarMinPort := b.portVal("ports.sidecar_min_port", c.Ports.SidecarMinPort)
	sidecarMaxPort := b.portVal("ports.sidecar_max_port", c.Ports.SidecarMaxPort)
	if proxyMaxPort < proxyMinPort {
		return RuntimeConfig{}, fmt.Errorf(
			"proxy_min_port must be less than proxy_max_port. To disable, set both to zero.")
	}
	if sidecarMaxPort < sidecarMinPort {
		return RuntimeConfig{}, fmt.Errorf(
			"sidecar_min_port must be less than sidecar_max_port. To disable, set both to zero.")
	}

	// determine the default bind and advertise address
	//
	// First check whether the user provided an ANY address or whether
	// the expanded template results in an ANY address. In that case we
	// derive an advertise address from the current network
	// configuration since we can listen on an ANY address for incoming
	// traffic but cannot advertise it as the address on which the
	// server can be reached.

	bindAddrs := b.expandAddrs("bind_addr", c.BindAddr)
	if len(bindAddrs) == 0 {
		return RuntimeConfig{}, fmt.Errorf("bind_addr cannot be empty")
	}
	if len(bindAddrs) > 1 {
		return RuntimeConfig{}, fmt.Errorf("bind_addr cannot contain multiple addresses. Use 'addresses.{dns,http,https}' instead.")
	}
	if isUnixAddr(bindAddrs[0]) {
		return RuntimeConfig{}, fmt.Errorf("bind_addr cannot be a unix socket")
	}
	if !isIPAddr(bindAddrs[0]) {
		return RuntimeConfig{}, fmt.Errorf("bind_addr must be an ip address")
	}
	if ipaddr.IsAny(b.stringVal(c.AdvertiseAddrLAN)) {
		return RuntimeConfig{}, fmt.Errorf("Advertise address cannot be 0.0.0.0, :: or [::]")
	}
	if ipaddr.IsAny(b.stringVal(c.AdvertiseAddrWAN)) {
		return RuntimeConfig{}, fmt.Errorf("Advertise WAN address cannot be 0.0.0.0, :: or [::]")
	}

	bindAddr := bindAddrs[0].(*net.IPAddr)
	advertiseAddr := b.makeIPAddr(b.expandFirstIP("advertise_addr", c.AdvertiseAddrLAN), bindAddr)
	if ipaddr.IsAny(advertiseAddr) {

		var addrtyp string
		var detect func() ([]*net.IPAddr, error)
		switch {
		case ipaddr.IsAnyV4(advertiseAddr):
			addrtyp = "private IPv4"
			detect = b.GetPrivateIPv4
			if detect == nil {
				detect = ipaddr.GetPrivateIPv4
			}

		case ipaddr.IsAnyV6(advertiseAddr):
			addrtyp = "public IPv6"
			detect = b.GetPublicIPv6
			if detect == nil {
				detect = ipaddr.GetPublicIPv6
			}
		}

		advertiseAddrs, err := detect()
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("Error detecting %s address: %s", addrtyp, err)
		}
		if len(advertiseAddrs) == 0 {
			return RuntimeConfig{}, fmt.Errorf("No %s address found", addrtyp)
		}
		if len(advertiseAddrs) > 1 {
			return RuntimeConfig{}, fmt.Errorf("Multiple %s addresses found. Please configure one with 'bind' and/or 'advertise'.", addrtyp)
		}
		advertiseAddr = advertiseAddrs[0]
	}

	// derive other bind addresses from the bindAddr
	rpcBindAddr := b.makeTCPAddr(bindAddr, nil, serverPort)
	serfBindAddrLAN := b.makeTCPAddr(b.expandFirstIP("serf_lan", c.SerfBindAddrLAN), bindAddr, serfPortLAN)

	// Only initialize serf WAN bind address when its enabled
	var serfBindAddrWAN *net.TCPAddr
	if serfPortWAN >= 0 {
		serfBindAddrWAN = b.makeTCPAddr(b.expandFirstIP("serf_wan", c.SerfBindAddrWAN), bindAddr, serfPortWAN)
	}

	// derive other advertise addresses from the advertise address
	advertiseAddrLAN := b.makeIPAddr(b.expandFirstIP("advertise_addr", c.AdvertiseAddrLAN), advertiseAddr)
	advertiseAddrWAN := b.makeIPAddr(b.expandFirstIP("advertise_addr_wan", c.AdvertiseAddrWAN), advertiseAddrLAN)
	rpcAdvertiseAddr := &net.TCPAddr{IP: advertiseAddrLAN.IP, Port: serverPort}
	serfAdvertiseAddrLAN := &net.TCPAddr{IP: advertiseAddrLAN.IP, Port: serfPortLAN}
	// Only initialize serf WAN advertise address when its enabled
	var serfAdvertiseAddrWAN *net.TCPAddr
	if serfPortWAN >= 0 {
		serfAdvertiseAddrWAN = &net.TCPAddr{IP: advertiseAddrWAN.IP, Port: serfPortWAN}
	}

	// determine client addresses
	clientAddrs := b.expandIPs("client_addr", c.ClientAddr)
	dnsAddrs := b.makeAddrs(b.expandAddrs("addresses.dns", c.Addresses.DNS), clientAddrs, dnsPort)
	httpAddrs := b.makeAddrs(b.expandAddrs("addresses.http", c.Addresses.HTTP), clientAddrs, httpPort)
	httpsAddrs := b.makeAddrs(b.expandAddrs("addresses.https", c.Addresses.HTTPS), clientAddrs, httpsPort)
	grpcAddrs := b.makeAddrs(b.expandAddrs("addresses.grpc", c.Addresses.GRPC), clientAddrs, grpcPort)

	for _, a := range dnsAddrs {
		if x, ok := a.(*net.TCPAddr); ok {
			dnsAddrs = append(dnsAddrs, &net.UDPAddr{IP: x.IP, Port: x.Port})
		}
	}

	// expand dns recursors
	uniq := map[string]bool{}
	dnsRecursors := []string{}
	for _, r := range c.DNSRecursors {
		x, err := template.Parse(r)
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("Invalid DNS recursor template %q: %s", r, err)
		}
		for _, addr := range strings.Fields(x) {
			if strings.HasPrefix(addr, "unix://") {
				return RuntimeConfig{}, fmt.Errorf("DNS Recursors cannot be unix sockets: %s", addr)
			}
			if uniq[addr] {
				continue
			}
			uniq[addr] = true
			dnsRecursors = append(dnsRecursors, addr)
		}
	}

	datacenter := strings.ToLower(b.stringVal(c.Datacenter))
	altDomain := b.stringVal(c.DNSAltDomain)

	// Create the default set of tagged addresses.
	if c.TaggedAddresses == nil {
		c.TaggedAddresses = make(map[string]string)
	}
	c.TaggedAddresses["lan"] = advertiseAddrLAN.IP.String()
	c.TaggedAddresses["wan"] = advertiseAddrWAN.IP.String()

	// segments
	var segments []structs.NetworkSegment
	for _, s := range c.Segments {
		name := b.stringVal(s.Name)
		port := b.portVal(fmt.Sprintf("segments[%s].port", name), s.Port)
		if port <= 0 {
			return RuntimeConfig{}, fmt.Errorf("Port for segment %q cannot be <= 0", name)
		}

		bind := b.makeTCPAddr(
			b.expandFirstIP(fmt.Sprintf("segments[%s].bind", name), s.Bind),
			bindAddr,
			port,
		)

		advertise := b.makeTCPAddr(
			b.expandFirstIP(fmt.Sprintf("segments[%s].advertise", name), s.Advertise),
			advertiseAddrLAN,
			port,
		)

		segments = append(segments, structs.NetworkSegment{
			Name:        name,
			Bind:        bind,
			Advertise:   advertise,
			RPCListener: b.boolVal(s.RPCListener),
		})
	}

	// Parse the metric filters
	var telemetryAllowedPrefixes, telemetryBlockedPrefixes []string
	for _, rule := range c.Telemetry.PrefixFilter {
		if rule == "" {
			b.warn("Cannot have empty filter rule in prefix_filter")
			continue
		}
		switch rule[0] {
		case '+':
			telemetryAllowedPrefixes = append(telemetryAllowedPrefixes, rule[1:])
		case '-':
			telemetryBlockedPrefixes = append(telemetryBlockedPrefixes, rule[1:])
		default:
			b.warn("Filter rule must begin with either '+' or '-': %q", rule)
		}
	}

	// raft performance scaling
	performanceRaftMultiplier := b.intVal(c.Performance.RaftMultiplier)
	if performanceRaftMultiplier < 1 || uint(performanceRaftMultiplier) > consul.MaxRaftMultiplier {
		return RuntimeConfig{}, fmt.Errorf("performance.raft_multiplier cannot be %d. Must be between 1 and %d", performanceRaftMultiplier, consul.MaxRaftMultiplier)
	}
	consulRaftElectionTimeout := b.durationVal("consul.raft.election_timeout", c.Consul.Raft.ElectionTimeout) * time.Duration(performanceRaftMultiplier)
	consulRaftHeartbeatTimeout := b.durationVal("consul.raft.heartbeat_timeout", c.Consul.Raft.HeartbeatTimeout) * time.Duration(performanceRaftMultiplier)
	consulRaftLeaderLeaseTimeout := b.durationVal("consul.raft.leader_lease_timeout", c.Consul.Raft.LeaderLeaseTimeout) * time.Duration(performanceRaftMultiplier)

	// Connect proxy defaults.
	connectEnabled := b.boolVal(c.Connect.Enabled)
	connectCAProvider := b.stringVal(c.Connect.CAProvider)
	connectCAConfig := c.Connect.CAConfig
	if connectCAConfig != nil {
		lib.TranslateKeys(connectCAConfig, map[string]string{
			// Consul CA config
			"private_key":     "PrivateKey",
			"root_cert":       "RootCert",
			"rotation_period": "RotationPeriod",

			// Vault CA config
			"address":               "Address",
			"token":                 "Token",
			"root_pki_path":         "RootPKIPath",
			"intermediate_pki_path": "IntermediatePKIPath",
			"ca_file":               "CAFile",
			"ca_path":               "CAPath",
			"cert_file":             "CertFile",
			"key_file":              "KeyFile",
			"tls_server_name":       "TLSServerName",
			"tls_skip_verify":       "TLSSkipVerify",

			// Common CA config
			"leaf_cert_ttl":      "LeafCertTTL",
			"csr_max_per_second": "CSRMaxPerSecond",
			"csr_max_concurrent": "CSRMaxConcurrent",
			"private_key_type":   "PrivateKeyType",
			"private_key_bits":   "PrivateKeyBits",
		})
	}

	autoEncryptTLS := b.boolVal(c.AutoEncrypt.TLS)
	autoEncryptAllowTLS := b.boolVal(c.AutoEncrypt.AllowTLS)

	if autoEncryptAllowTLS {
		connectEnabled = true
	}

	aclsEnabled := false
	primaryDatacenter := strings.ToLower(b.stringVal(c.PrimaryDatacenter))
	if c.ACLDatacenter != nil {
		b.warn("The 'acl_datacenter' field is deprecated. Use the 'primary_datacenter' field instead.")

		if primaryDatacenter == "" {
			primaryDatacenter = strings.ToLower(b.stringVal(c.ACLDatacenter))
		}

		// when the acl_datacenter config is used it implicitly enables acls
		aclsEnabled = true
	}

	if c.ACL.Enabled != nil {
		aclsEnabled = b.boolVal(c.ACL.Enabled)
	}

	aclDC := primaryDatacenter
	if aclsEnabled && aclDC == "" {
		aclDC = datacenter
	}

	enableTokenReplication := false
	if c.ACLReplicationToken != nil {
		enableTokenReplication = true
	}

	b.boolValWithDefault(c.ACL.TokenReplication, b.boolValWithDefault(c.EnableACLReplication, enableTokenReplication))

	enableRemoteScriptChecks := b.boolVal(c.EnableScriptChecks)
	enableLocalScriptChecks := b.boolValWithDefault(c.EnableLocalScriptChecks, enableRemoteScriptChecks)

	// VerifyServerHostname implies VerifyOutgoing
	verifyServerName := b.boolVal(c.VerifyServerHostname)
	verifyOutgoing := b.boolVal(c.VerifyOutgoing)
	if verifyServerName {
		// Setting only verify_server_hostname is documented to imply
		// verify_outgoing. If it doesn't then we risk sending communication over TCP
		// when we documented it as forcing TLS for RPCs. Enforce this here rather
		// than in several different places through the code that need to reason
		// about it. (See CVE-2018-19653)
		verifyOutgoing = true
	}

	var configEntries []structs.ConfigEntry

	if len(c.ConfigEntries.Bootstrap) > 0 {
		for i, rawEntry := range c.ConfigEntries.Bootstrap {
			entry, err := structs.DecodeConfigEntry(rawEntry)
			if err != nil {
				return RuntimeConfig{}, fmt.Errorf("config_entries.bootstrap[%d]: %s", i, err)
			}
			if err := entry.Validate(); err != nil {
				return RuntimeConfig{}, fmt.Errorf("config_entries.bootstrap[%d]: %s", i, err)
			}
			configEntries = append(configEntries, entry)
		}
	}

	// ----------------------------------------------------------------
	// build runtime config
	//
	rt = RuntimeConfig{
		// non-user configurable values
		ACLDisabledTTL:             b.durationVal("acl.disabled_ttl", c.ACL.DisabledTTL),
		AEInterval:                 b.durationVal("ae_interval", c.AEInterval),
		CheckDeregisterIntervalMin: b.durationVal("check_deregister_interval_min", c.CheckDeregisterIntervalMin),
		CheckReapInterval:          b.durationVal("check_reap_interval", c.CheckReapInterval),
		Revision:                   b.stringVal(c.Revision),
		SegmentLimit:               b.intVal(c.SegmentLimit),
		SegmentNameLimit:           b.intVal(c.SegmentNameLimit),
		SyncCoordinateIntervalMin:  b.durationVal("sync_coordinate_interval_min", c.SyncCoordinateIntervalMin),
		SyncCoordinateRateTarget:   b.float64Val(c.SyncCoordinateRateTarget),
		Version:                    b.stringVal(c.Version),
		VersionPrerelease:          b.stringVal(c.VersionPrerelease),

		// consul configuration
		ConsulCoordinateUpdateBatchSize:  b.intVal(c.Consul.Coordinate.UpdateBatchSize),
		ConsulCoordinateUpdateMaxBatches: b.intVal(c.Consul.Coordinate.UpdateMaxBatches),
		ConsulCoordinateUpdatePeriod:     b.durationVal("consul.coordinate.update_period", c.Consul.Coordinate.UpdatePeriod),
		ConsulRaftElectionTimeout:        consulRaftElectionTimeout,
		ConsulRaftHeartbeatTimeout:       consulRaftHeartbeatTimeout,
		ConsulRaftLeaderLeaseTimeout:     consulRaftLeaderLeaseTimeout,
		ConsulServerHealthInterval:       b.durationVal("consul.server.health_interval", c.Consul.Server.HealthInterval),

		// gossip configuration
		GossipLANGossipInterval: b.durationVal("gossip_lan..gossip_interval", c.GossipLAN.GossipInterval),
		GossipLANGossipNodes:    b.intVal(c.GossipLAN.GossipNodes),
		GossipLANProbeInterval:  b.durationVal("gossip_lan..probe_interval", c.GossipLAN.ProbeInterval),
		GossipLANProbeTimeout:   b.durationVal("gossip_lan..probe_timeout", c.GossipLAN.ProbeTimeout),
		GossipLANSuspicionMult:  b.intVal(c.GossipLAN.SuspicionMult),
		GossipLANRetransmitMult: b.intVal(c.GossipLAN.RetransmitMult),
		GossipWANGossipInterval: b.durationVal("gossip_wan..gossip_interval", c.GossipWAN.GossipInterval),
		GossipWANGossipNodes:    b.intVal(c.GossipWAN.GossipNodes),
		GossipWANProbeInterval:  b.durationVal("gossip_wan..probe_interval", c.GossipWAN.ProbeInterval),
		GossipWANProbeTimeout:   b.durationVal("gossip_wan..probe_timeout", c.GossipWAN.ProbeTimeout),
		GossipWANSuspicionMult:  b.intVal(c.GossipWAN.SuspicionMult),
		GossipWANRetransmitMult: b.intVal(c.GossipWAN.RetransmitMult),

		// ACL
		ACLEnforceVersion8:        b.boolValWithDefault(c.ACLEnforceVersion8, true),
		ACLsEnabled:               aclsEnabled,
		ACLAgentMasterToken:       b.stringValWithDefault(c.ACL.Tokens.AgentMaster, b.stringVal(c.ACLAgentMasterToken)),
		ACLAgentToken:             b.stringValWithDefault(c.ACL.Tokens.Agent, b.stringVal(c.ACLAgentToken)),
		ACLDatacenter:             aclDC,
		ACLDefaultPolicy:          b.stringValWithDefault(c.ACL.DefaultPolicy, b.stringVal(c.ACLDefaultPolicy)),
		ACLDownPolicy:             b.stringValWithDefault(c.ACL.DownPolicy, b.stringVal(c.ACLDownPolicy)),
		ACLEnableKeyListPolicy:    b.boolValWithDefault(c.ACL.EnableKeyListPolicy, b.boolVal(c.ACLEnableKeyListPolicy)),
		ACLMasterToken:            b.stringValWithDefault(c.ACL.Tokens.Master, b.stringVal(c.ACLMasterToken)),
		ACLReplicationToken:       b.stringValWithDefault(c.ACL.Tokens.Replication, b.stringVal(c.ACLReplicationToken)),
		ACLTokenTTL:               b.durationValWithDefault("acl.token_ttl", c.ACL.TokenTTL, b.durationVal("acl_ttl", c.ACLTTL)),
		ACLPolicyTTL:              b.durationVal("acl.policy_ttl", c.ACL.PolicyTTL),
		ACLRoleTTL:                b.durationVal("acl.role_ttl", c.ACL.RoleTTL),
		ACLToken:                  b.stringValWithDefault(c.ACL.Tokens.Default, b.stringVal(c.ACLToken)),
		ACLTokenReplication:       b.boolValWithDefault(c.ACL.TokenReplication, b.boolValWithDefault(c.EnableACLReplication, enableTokenReplication)),
		ACLEnableTokenPersistence: b.boolValWithDefault(c.ACL.EnableTokenPersistence, false),

		// Autopilot
		AutopilotCleanupDeadServers:      b.boolVal(c.Autopilot.CleanupDeadServers),
		AutopilotDisableUpgradeMigration: b.boolVal(c.Autopilot.DisableUpgradeMigration),
		AutopilotLastContactThreshold:    b.durationVal("autopilot.last_contact_threshold", c.Autopilot.LastContactThreshold),
		AutopilotMaxTrailingLogs:         b.intVal(c.Autopilot.MaxTrailingLogs),
		AutopilotRedundancyZoneTag:       b.stringVal(c.Autopilot.RedundancyZoneTag),
		AutopilotServerStabilizationTime: b.durationVal("autopilot.server_stabilization_time", c.Autopilot.ServerStabilizationTime),
		AutopilotUpgradeVersionTag:       b.stringVal(c.Autopilot.UpgradeVersionTag),

		// DNS
		DNSAddrs:              dnsAddrs,
		DNSAllowStale:         b.boolVal(c.DNS.AllowStale),
		DNSARecordLimit:       b.intVal(c.DNS.ARecordLimit),
		DNSDisableCompression: b.boolVal(c.DNS.DisableCompression),
		DNSDomain:             b.stringVal(c.DNSDomain),
		DNSAltDomain:          altDomain,
		DNSEnableTruncate:     b.boolVal(c.DNS.EnableTruncate),
		DNSMaxStale:           b.durationVal("dns_config.max_stale", c.DNS.MaxStale),
		DNSNodeTTL:            b.durationVal("dns_config.node_ttl", c.DNS.NodeTTL),
		DNSOnlyPassing:        b.boolVal(c.DNS.OnlyPassing),
		DNSPort:               dnsPort,
		DNSRecursorTimeout:    b.durationVal("recursor_timeout", c.DNS.RecursorTimeout),
		DNSRecursors:          dnsRecursors,
		DNSServiceTTL:         dnsServiceTTL,
		DNSSOA:                soa,
		DNSUDPAnswerLimit:     b.intVal(c.DNS.UDPAnswerLimit),
		DNSNodeMetaTXT:        b.boolValWithDefault(c.DNS.NodeMetaTXT, true),
		DNSUseCache:           b.boolVal(c.DNS.UseCache),
		DNSCacheMaxAge:        b.durationVal("dns_config.cache_max_age", c.DNS.CacheMaxAge),

		// HTTP
		HTTPPort:            httpPort,
		HTTPSPort:           httpsPort,
		HTTPAddrs:           httpAddrs,
		HTTPSAddrs:          httpsAddrs,
		HTTPBlockEndpoints:  c.HTTPConfig.BlockEndpoints,
		HTTPResponseHeaders: c.HTTPConfig.ResponseHeaders,
		AllowWriteHTTPFrom:  b.cidrsVal("allow_write_http_from", c.HTTPConfig.AllowWriteHTTPFrom),

		// Telemetry
		Telemetry: lib.TelemetryConfig{
			CirconusAPIApp:                     b.stringVal(c.Telemetry.CirconusAPIApp),
			CirconusAPIToken:                   b.stringVal(c.Telemetry.CirconusAPIToken),
			CirconusAPIURL:                     b.stringVal(c.Telemetry.CirconusAPIURL),
			CirconusBrokerID:                   b.stringVal(c.Telemetry.CirconusBrokerID),
			CirconusBrokerSelectTag:            b.stringVal(c.Telemetry.CirconusBrokerSelectTag),
			CirconusCheckDisplayName:           b.stringVal(c.Telemetry.CirconusCheckDisplayName),
			CirconusCheckForceMetricActivation: b.stringVal(c.Telemetry.CirconusCheckForceMetricActivation),
			CirconusCheckID:                    b.stringVal(c.Telemetry.CirconusCheckID),
			CirconusCheckInstanceID:            b.stringVal(c.Telemetry.CirconusCheckInstanceID),
			CirconusCheckSearchTag:             b.stringVal(c.Telemetry.CirconusCheckSearchTag),
			CirconusCheckTags:                  b.stringVal(c.Telemetry.CirconusCheckTags),
			CirconusSubmissionInterval:         b.stringVal(c.Telemetry.CirconusSubmissionInterval),
			CirconusSubmissionURL:              b.stringVal(c.Telemetry.CirconusSubmissionURL),
			DisableHostname:                    b.boolVal(c.Telemetry.DisableHostname),
			DogstatsdAddr:                      b.stringVal(c.Telemetry.DogstatsdAddr),
			DogstatsdTags:                      c.Telemetry.DogstatsdTags,
			PrometheusRetentionTime:            b.durationVal("prometheus_retention_time", c.Telemetry.PrometheusRetentionTime),
			FilterDefault:                      b.boolVal(c.Telemetry.FilterDefault),
			AllowedPrefixes:                    telemetryAllowedPrefixes,
			BlockedPrefixes:                    telemetryBlockedPrefixes,
			MetricsPrefix:                      b.stringVal(c.Telemetry.MetricsPrefix),
			StatsdAddr:                         b.stringVal(c.Telemetry.StatsdAddr),
			StatsiteAddr:                       b.stringVal(c.Telemetry.StatsiteAddr),
		},

		// Agent
		AdvertiseAddrLAN:                 advertiseAddrLAN,
		AdvertiseAddrWAN:                 advertiseAddrWAN,
		BindAddr:                         bindAddr,
		Bootstrap:                        b.boolVal(c.Bootstrap),
		BootstrapExpect:                  b.intVal(c.BootstrapExpect),
		CAFile:                           b.stringVal(c.CAFile),
		CAPath:                           b.stringVal(c.CAPath),
		CertFile:                         b.stringVal(c.CertFile),
		CheckUpdateInterval:              b.durationVal("check_update_interval", c.CheckUpdateInterval),
		CheckOutputMaxSize:               b.intValWithDefault(c.CheckOutputMaxSize, 4096),
		Checks:                           checks,
		ClientAddrs:                      clientAddrs,
		ConfigEntryBootstrap:             configEntries,
		AutoEncryptTLS:                   autoEncryptTLS,
		AutoEncryptAllowTLS:              autoEncryptAllowTLS,
		ConnectEnabled:                   connectEnabled,
		ConnectCAProvider:                connectCAProvider,
		ConnectCAConfig:                  connectCAConfig,
		ConnectSidecarMinPort:            sidecarMinPort,
		ConnectSidecarMaxPort:            sidecarMaxPort,
		DataDir:                          b.stringVal(c.DataDir),
		Datacenter:                       datacenter,
		DevMode:                          b.boolVal(b.Flags.DevMode),
		DisableAnonymousSignature:        b.boolVal(c.DisableAnonymousSignature),
		DisableCoordinates:               b.boolVal(c.DisableCoordinates),
		DisableHostNodeID:                b.boolVal(c.DisableHostNodeID),
		DisableHTTPUnprintableCharFilter: b.boolVal(c.DisableHTTPUnprintableCharFilter),
		DisableKeyringFile:               b.boolVal(c.DisableKeyringFile),
		DisableRemoteExec:                b.boolVal(c.DisableRemoteExec),
		DisableUpdateCheck:               b.boolVal(c.DisableUpdateCheck),
		DiscardCheckOutput:               b.boolVal(c.DiscardCheckOutput),
		DiscoveryMaxStale:                b.durationVal("discovery_max_stale", c.DiscoveryMaxStale),
		EnableAgentTLSForChecks:          b.boolVal(c.EnableAgentTLSForChecks),
		EnableCentralServiceConfig:       b.boolVal(c.EnableCentralServiceConfig),
		EnableDebug:                      b.boolVal(c.EnableDebug),
		EnableRemoteScriptChecks:         enableRemoteScriptChecks,
		EnableLocalScriptChecks:          enableLocalScriptChecks,
		EnableSyslog:                     b.boolVal(c.EnableSyslog),
		EnableUI:                         b.boolVal(c.UI),
		EncryptKey:                       b.stringVal(c.EncryptKey),
		EncryptVerifyIncoming:            b.boolVal(c.EncryptVerifyIncoming),
		EncryptVerifyOutgoing:            b.boolVal(c.EncryptVerifyOutgoing),
		GRPCPort:                         grpcPort,
		GRPCAddrs:                        grpcAddrs,
		KeyFile:                          b.stringVal(c.KeyFile),
		KVMaxValueSize:                   b.uint64Val(c.Limits.KVMaxValueSize),
		LeaveDrainTime:                   b.durationVal("performance.leave_drain_time", c.Performance.LeaveDrainTime),
		LeaveOnTerm:                      leaveOnTerm,
		LogLevel:                         b.stringVal(c.LogLevel),
		LogFile:                          b.stringVal(c.LogFile),
		LogRotateBytes:                   b.intVal(c.LogRotateBytes),
		LogRotateDuration:                b.durationVal("log_rotate_duration", c.LogRotateDuration),
		LogRotateMaxFiles:                b.intVal(c.LogRotateMaxFiles),
		NodeID:                           types.NodeID(b.stringVal(c.NodeID)),
		NodeMeta:                         c.NodeMeta,
		NodeName:                         b.nodeName(c.NodeName),
		NonVotingServer:                  b.boolVal(c.NonVotingServer),
		PidFile:                          b.stringVal(c.PidFile),
		PrimaryDatacenter:                primaryDatacenter,
		RPCAdvertiseAddr:                 rpcAdvertiseAddr,
		RPCBindAddr:                      rpcBindAddr,
		RPCHoldTimeout:                   b.durationVal("performance.rpc_hold_timeout", c.Performance.RPCHoldTimeout),
		RPCMaxBurst:                      b.intVal(c.Limits.RPCMaxBurst),
		RPCProtocol:                      b.intVal(c.RPCProtocol),
		RPCRateLimit:                     rate.Limit(b.float64Val(c.Limits.RPCRate)),
		RaftProtocol:                     b.intVal(c.RaftProtocol),
		RaftSnapshotThreshold:            b.intVal(c.RaftSnapshotThreshold),
		RaftSnapshotInterval:             b.durationVal("raft_snapshot_interval", c.RaftSnapshotInterval),
		RaftTrailingLogs:                 b.intVal(c.RaftTrailingLogs),
		ReconnectTimeoutLAN:              b.durationVal("reconnect_timeout", c.ReconnectTimeoutLAN),
		ReconnectTimeoutWAN:              b.durationVal("reconnect_timeout_wan", c.ReconnectTimeoutWAN),
		RejoinAfterLeave:                 b.boolVal(c.RejoinAfterLeave),
		RetryJoinIntervalLAN:             b.durationVal("retry_interval", c.RetryJoinIntervalLAN),
		RetryJoinIntervalWAN:             b.durationVal("retry_interval_wan", c.RetryJoinIntervalWAN),
		RetryJoinLAN:                     b.expandAllOptionalAddrs("retry_join", c.RetryJoinLAN),
		RetryJoinMaxAttemptsLAN:          b.intVal(c.RetryJoinMaxAttemptsLAN),
		RetryJoinMaxAttemptsWAN:          b.intVal(c.RetryJoinMaxAttemptsWAN),
		RetryJoinWAN:                     b.expandAllOptionalAddrs("retry_join_wan", c.RetryJoinWAN),
		SegmentName:                      b.stringVal(c.SegmentName),
		Segments:                         segments,
		SerfAdvertiseAddrLAN:             serfAdvertiseAddrLAN,
		SerfAdvertiseAddrWAN:             serfAdvertiseAddrWAN,
		SerfBindAddrLAN:                  serfBindAddrLAN,
		SerfBindAddrWAN:                  serfBindAddrWAN,
		SerfPortLAN:                      serfPortLAN,
		SerfPortWAN:                      serfPortWAN,
		ServerMode:                       b.boolVal(c.ServerMode),
		ServerName:                       b.stringVal(c.ServerName),
		ServerPort:                       serverPort,
		Services:                         services,
		SessionTTLMin:                    b.durationVal("session_ttl_min", c.SessionTTLMin),
		SkipLeaveOnInt:                   skipLeaveOnInt,
		StartJoinAddrsLAN:                b.expandAllOptionalAddrs("start_join", c.StartJoinAddrsLAN),
		StartJoinAddrsWAN:                b.expandAllOptionalAddrs("start_join_wan", c.StartJoinAddrsWAN),
		SyslogFacility:                   b.stringVal(c.SyslogFacility),
		TLSCipherSuites:                  b.tlsCipherSuites("tls_cipher_suites", c.TLSCipherSuites),
		TLSMinVersion:                    b.stringVal(c.TLSMinVersion),
		TLSPreferServerCipherSuites:      b.boolVal(c.TLSPreferServerCipherSuites),
		TaggedAddresses:                  c.TaggedAddresses,
		TranslateWANAddrs:                b.boolVal(c.TranslateWANAddrs),
		UIDir:                            b.stringVal(c.UIDir),
		UIContentPath:                    UIPathBuilder(b.stringVal(b.Flags.Config.UIContentPath)),
		UnixSocketGroup:                  b.stringVal(c.UnixSocket.Group),
		UnixSocketMode:                   b.stringVal(c.UnixSocket.Mode),
		UnixSocketUser:                   b.stringVal(c.UnixSocket.User),
		VerifyIncoming:                   b.boolVal(c.VerifyIncoming),
		VerifyIncomingHTTPS:              b.boolVal(c.VerifyIncomingHTTPS),
		VerifyIncomingRPC:                b.boolVal(c.VerifyIncomingRPC),
		VerifyOutgoing:                   verifyOutgoing,
		VerifyServerHostname:             verifyServerName,
		Watches:                          c.Watches,
	}

	if rt.BootstrapExpect == 1 {
		rt.Bootstrap = true
		rt.BootstrapExpect = 0
		b.warn(`BootstrapExpect is set to 1; this is the same as Bootstrap mode.`)
	}

	return rt, nil
}

// Validate performs semantical validation of the runtime configuration.
func (b *Builder) Validate(rt RuntimeConfig) error {
	// reDatacenter defines a regexp for a valid datacenter name
	var reDatacenter = regexp.MustCompile("^[a-z0-9_-]+$")

	// validContentPath defines a regexp for a valid content path name.
	var validContentPath = regexp.MustCompile(`^[A-Za-z0-9/_-]+$`)
	var hasVersion = regexp.MustCompile(`^/v\d+/$`)
	// ----------------------------------------------------------------
	// check required params we cannot recover from first
	//

	if rt.Datacenter == "" {
		return fmt.Errorf("datacenter cannot be empty")
	}
	if !reDatacenter.MatchString(rt.Datacenter) {
		return fmt.Errorf("datacenter cannot be %q. Please use only [a-z0-9-_]", rt.Datacenter)
	}
	if rt.DataDir == "" && !rt.DevMode {
		return fmt.Errorf("data_dir cannot be empty")
	}

	if !validContentPath.MatchString(rt.UIContentPath) {
		return fmt.Errorf("ui-content-path can only contain alphanumeric, -, _, or /. received: %s", rt.UIContentPath)
	}

	if hasVersion.MatchString(rt.UIContentPath) {
		return fmt.Errorf("ui-content-path cannot have 'v[0-9]'. received: %s", rt.UIContentPath)
	}

	if !rt.DevMode {
		fi, err := os.Stat(rt.DataDir)
		switch {
		case err != nil && !os.IsNotExist(err):
			return fmt.Errorf("Error getting info on data_dir: %s", err)
		case err == nil && !fi.IsDir():
			return fmt.Errorf("data_dir %q is not a directory", rt.DataDir)
		}
	}
	if rt.NodeName == "" {
		return fmt.Errorf("node_name cannot be empty")
	}
	if ipaddr.IsAny(rt.AdvertiseAddrLAN.IP) {
		return fmt.Errorf("Advertise address cannot be 0.0.0.0, :: or [::]")
	}
	if ipaddr.IsAny(rt.AdvertiseAddrWAN.IP) {
		return fmt.Errorf("Advertise WAN address cannot be 0.0.0.0, :: or [::]")
	}
	if err := b.validateSegments(rt); err != nil {
		return err
	}
	for _, a := range rt.DNSAddrs {
		if _, ok := a.(*net.UnixAddr); ok {
			return fmt.Errorf("DNS address cannot be a unix socket")
		}
	}
	for _, a := range rt.DNSRecursors {
		if ipaddr.IsAny(a) {
			return fmt.Errorf("DNS recursor address cannot be 0.0.0.0, :: or [::]")
		}
	}
	if !isValidAltDomain(rt.DNSAltDomain, rt.Datacenter) {
		return fmt.Errorf("alt_domain cannot start with {service,connect,node,query,addr,%s}", rt.Datacenter)
	}
	if rt.Bootstrap && !rt.ServerMode {
		return fmt.Errorf("'bootstrap = true' requires 'server = true'")
	}
	if rt.BootstrapExpect < 0 {
		return fmt.Errorf("bootstrap_expect cannot be %d. Must be greater than or equal to zero", rt.BootstrapExpect)
	}
	if rt.BootstrapExpect > 0 && !rt.ServerMode {
		return fmt.Errorf("'bootstrap_expect > 0' requires 'server = true'")
	}
	if rt.BootstrapExpect > 0 && rt.DevMode {
		return fmt.Errorf("'bootstrap_expect > 0' not allowed in dev mode")
	}
	if rt.BootstrapExpect > 0 && rt.Bootstrap {
		return fmt.Errorf("'bootstrap_expect > 0' and 'bootstrap = true' are mutually exclusive")
	}
	if rt.CheckOutputMaxSize < 1 {
		return fmt.Errorf("check_output_max_size must be positive, to discard check output use the discard_check_output flag")
	}
	if rt.AEInterval <= 0 {
		return fmt.Errorf("ae_interval cannot be %s. Must be positive", rt.AEInterval)
	}
	if rt.AutopilotMaxTrailingLogs < 0 {
		return fmt.Errorf("autopilot.max_trailing_logs cannot be %d. Must be greater than or equal to zero", rt.AutopilotMaxTrailingLogs)
	}
	if rt.ACLDatacenter != "" && !reDatacenter.MatchString(rt.ACLDatacenter) {
		return fmt.Errorf("acl_datacenter cannot be %q. Please use only [a-z0-9-_]", rt.ACLDatacenter)
	}
	if rt.EnableUI && rt.UIDir != "" {
		return fmt.Errorf(
			"Both the ui and ui-dir flags were specified, please provide only one.\n" +
				"If trying to use your own web UI resources, use the ui-dir flag.\n" +
				"If using Consul version 0.7.0 or later, the web UI is included in the binary so use ui to enable it")
	}
	if rt.DNSUDPAnswerLimit < 0 {
		return fmt.Errorf("dns_config.udp_answer_limit cannot be %d. Must be greater than or equal to zero", rt.DNSUDPAnswerLimit)
	}
	if rt.DNSARecordLimit < 0 {
		return fmt.Errorf("dns_config.a_record_limit cannot be %d. Must be greater than or equal to zero", rt.DNSARecordLimit)
	}
	if err := structs.ValidateMetadata(rt.NodeMeta, false); err != nil {
		return fmt.Errorf("node_meta invalid: %v", err)
	}
	if rt.EncryptKey != "" {
		if _, err := decodeBytes(rt.EncryptKey); err != nil {
			return fmt.Errorf("encrypt has invalid key: %s", err)
		}
		keyfileLAN := filepath.Join(rt.DataDir, SerfLANKeyring)
		if _, err := os.Stat(keyfileLAN); err == nil {
			b.warn("WARNING: LAN keyring exists but -encrypt given, using keyring")
		}
		if rt.ServerMode {
			keyfileWAN := filepath.Join(rt.DataDir, SerfWANKeyring)
			if _, err := os.Stat(keyfileWAN); err == nil {
				b.warn("WARNING: WAN keyring exists but -encrypt given, using keyring")
			}
		}
	}

	// Check the data dir for signs of an un-migrated Consul 0.5.x or older
	// server. Consul refuses to start if this is present to protect a server
	// with existing data from starting on a fresh data set.
	if rt.ServerMode {
		mdbPath := filepath.Join(rt.DataDir, "mdb")
		if _, err := os.Stat(mdbPath); !os.IsNotExist(err) {
			if os.IsPermission(err) {
				return fmt.Errorf(
					"CRITICAL: Permission denied for data folder at %q!\n"+
						"Consul will refuse to boot without access to this directory.\n"+
						"Please correct permissions and try starting again.", mdbPath)
			}
			return fmt.Errorf("CRITICAL: Deprecated data folder found at %q!\n"+
				"Consul will refuse to boot with this directory present.\n"+
				"See https://www.consul.io/docs/upgrade-specific.html for more information.", mdbPath)
		}
	}

	inuse := map[string]string{}
	if err := addrsUnique(inuse, "DNS", rt.DNSAddrs); err != nil {
		// cannot happen since this is the first address
		// we leave this for consistency
		return err
	}
	if err := addrsUnique(inuse, "HTTP", rt.HTTPAddrs); err != nil {
		return err
	}
	if err := addrsUnique(inuse, "HTTPS", rt.HTTPSAddrs); err != nil {
		return err
	}
	if err := addrUnique(inuse, "RPC Advertise", rt.RPCAdvertiseAddr); err != nil {
		return err
	}
	if err := addrUnique(inuse, "Serf Advertise LAN", rt.SerfAdvertiseAddrLAN); err != nil {
		return err
	}
	// Validate serf WAN advertise address only when its set
	if rt.SerfAdvertiseAddrWAN != nil {
		if err := addrUnique(inuse, "Serf Advertise WAN", rt.SerfAdvertiseAddrWAN); err != nil {
			return err
		}
	}
	if b.err != nil {
		return b.err
	}

	// Check for errors in the service definitions
	for _, s := range rt.Services {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("service %q: %s", s.Name, err)
		}
	}

	// Validate the given Connect CA provider config
	validCAProviders := map[string]bool{
		"":                       true,
		structs.ConsulCAProvider: true,
		structs.VaultCAProvider:  true,
	}
	if _, ok := validCAProviders[rt.ConnectCAProvider]; !ok {
		return fmt.Errorf("%s is not a valid CA provider", rt.ConnectCAProvider)
	} else {
		switch rt.ConnectCAProvider {
		case structs.ConsulCAProvider:
			if _, err := ca.ParseConsulCAConfig(rt.ConnectCAConfig); err != nil {
				return err
			}
		case structs.VaultCAProvider:
			if _, err := ca.ParseVaultCAConfig(rt.ConnectCAConfig); err != nil {
				return err
			}
		}
	}

	if rt.AutoEncryptAllowTLS {
		if !rt.VerifyIncoming {
			return fmt.Errorf("if auto_encrypt.allow_tls is turned on, TLS must be configured in order to work properly.")
		}
	}

	// ----------------------------------------------------------------
	// warnings
	//

	if rt.ServerMode && !rt.DevMode && !rt.Bootstrap && rt.BootstrapExpect == 2 {
		b.warn(`bootstrap_expect = 2: A cluster with 2 servers will provide no failure tolerance. See https://www.consul.io/docs/internals/consensus.html#deployment-table`)
	}

	if rt.ServerMode && !rt.Bootstrap && rt.BootstrapExpect > 2 && rt.BootstrapExpect%2 == 0 {
		b.warn(`bootstrap_expect is even number: A cluster with an even number of servers does not achieve optimum fault tolerance. See https://www.consul.io/docs/internals/consensus.html#deployment-table`)
	}

	if rt.ServerMode && rt.Bootstrap && rt.BootstrapExpect == 0 {
		b.warn(`bootstrap = true: do not enable unless necessary`)
	}

	if rt.ServerMode && !rt.DevMode && !rt.Bootstrap && rt.BootstrapExpect > 1 {
		b.warn("bootstrap_expect > 0: expecting %d servers", rt.BootstrapExpect)
	}

	return nil
}

// addrUnique checks if the given address is already in use for another
// protocol.
func addrUnique(inuse map[string]string, name string, addr net.Addr) error {
	key := addr.Network() + ":" + addr.String()
	if other, ok := inuse[key]; ok {
		return fmt.Errorf("%s address %s already configured for %s", name, addr.String(), other)
	}
	inuse[key] = name
	return nil
}

// addrsUnique checks if any of the give addresses is already in use for
// another protocol.
func addrsUnique(inuse map[string]string, name string, addrs []net.Addr) error {
	for _, a := range addrs {
		if err := addrUnique(inuse, name, a); err != nil {
			return err
		}
	}
	return nil
}

// splitSlicesAndValues moves all slice values defined in c to 'slices'
// and all other values to 'values'.
func (b *Builder) splitSlicesAndValues(c Config) (slices, values Config) {
	v, t := reflect.ValueOf(c), reflect.TypeOf(c)
	rs, rv := reflect.New(t), reflect.New(t)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type.Kind() == reflect.Slice {
			rs.Elem().Field(i).Set(v.Field(i))
		} else {
			rv.Elem().Field(i).Set(v.Field(i))
		}
	}
	return rs.Elem().Interface().(Config), rv.Elem().Interface().(Config)
}

func (b *Builder) warn(msg string, args ...interface{}) {
	b.Warnings = append(b.Warnings, fmt.Sprintf(msg, args...))
}

func (b *Builder) checkVal(v *CheckDefinition) *structs.CheckDefinition {
	if v == nil {
		return nil
	}

	id := types.CheckID(b.stringVal(v.ID))

	return &structs.CheckDefinition{
		ID:                             id,
		Name:                           b.stringVal(v.Name),
		Notes:                          b.stringVal(v.Notes),
		ServiceID:                      b.stringVal(v.ServiceID),
		Token:                          b.stringVal(v.Token),
		Status:                         b.stringVal(v.Status),
		ScriptArgs:                     v.ScriptArgs,
		HTTP:                           b.stringVal(v.HTTP),
		Header:                         v.Header,
		Method:                         b.stringVal(v.Method),
		TCP:                            b.stringVal(v.TCP),
		Interval:                       b.durationVal(fmt.Sprintf("check[%s].interval", id), v.Interval),
		DockerContainerID:              b.stringVal(v.DockerContainerID),
		Shell:                          b.stringVal(v.Shell),
		GRPC:                           b.stringVal(v.GRPC),
		GRPCUseTLS:                     b.boolVal(v.GRPCUseTLS),
		TLSSkipVerify:                  b.boolVal(v.TLSSkipVerify),
		AliasNode:                      b.stringVal(v.AliasNode),
		AliasService:                   b.stringVal(v.AliasService),
		Timeout:                        b.durationVal(fmt.Sprintf("check[%s].timeout", id), v.Timeout),
		TTL:                            b.durationVal(fmt.Sprintf("check[%s].ttl", id), v.TTL),
		DeregisterCriticalServiceAfter: b.durationVal(fmt.Sprintf("check[%s].deregister_critical_service_after", id), v.DeregisterCriticalServiceAfter),
		OutputMaxSize:                  b.intValWithDefault(v.OutputMaxSize, checks.DefaultBufSize),
	}
}

func (b *Builder) svcTaggedAddresses(v map[string]ServiceAddress) map[string]structs.ServiceAddress {
	if len(v) <= 0 {
		return nil
	}

	svcAddrs := make(map[string]structs.ServiceAddress)
	for addrName, addrConf := range v {
		addr := structs.ServiceAddress{}
		if addrConf.Address != nil {
			addr.Address = *addrConf.Address
		}
		if addrConf.Port != nil {
			addr.Port = *addrConf.Port
		}

		svcAddrs[addrName] = addr
	}
	return svcAddrs
}

func (b *Builder) serviceVal(v *ServiceDefinition) *structs.ServiceDefinition {
	if v == nil {
		return nil
	}

	var checks structs.CheckTypes
	for _, check := range v.Checks {
		checks = append(checks, b.checkVal(&check).CheckType())
	}
	if v.Check != nil {
		checks = append(checks, b.checkVal(v.Check).CheckType())
	}

	meta := make(map[string]string)
	if err := structs.ValidateMetadata(v.Meta, false); err != nil {
		b.err = multierror.Append(fmt.Errorf("invalid meta for service %s: %v", b.stringVal(v.Name), err))
	} else {
		meta = v.Meta
	}
	serviceWeights := &structs.Weights{Passing: 1, Warning: 1}
	if v.Weights != nil {
		if v.Weights.Passing != nil {
			serviceWeights.Passing = *v.Weights.Passing
		}
		if v.Weights.Warning != nil {
			serviceWeights.Warning = *v.Weights.Warning
		}
	}

	if err := structs.ValidateWeights(serviceWeights); err != nil {
		b.err = multierror.Append(fmt.Errorf("Invalid weight definition for service %s: %s", b.stringVal(v.Name), err))
	}
	return &structs.ServiceDefinition{
		Kind:              b.serviceKindVal(v.Kind),
		ID:                b.stringVal(v.ID),
		Name:              b.stringVal(v.Name),
		Tags:              v.Tags,
		Address:           b.stringVal(v.Address),
		TaggedAddresses:   b.svcTaggedAddresses(v.TaggedAddresses),
		Meta:              meta,
		Port:              b.intVal(v.Port),
		Token:             b.stringVal(v.Token),
		EnableTagOverride: b.boolVal(v.EnableTagOverride),
		Weights:           serviceWeights,
		Checks:            checks,
		Proxy:             b.serviceProxyVal(v.Proxy),
		Connect:           b.serviceConnectVal(v.Connect),
	}
}

func (b *Builder) serviceKindVal(v *string) structs.ServiceKind {
	if v == nil {
		return structs.ServiceKindTypical
	}
	switch *v {
	case string(structs.ServiceKindConnectProxy):
		return structs.ServiceKindConnectProxy
	case string(structs.ServiceKindMeshGateway):
		return structs.ServiceKindMeshGateway
	default:
		return structs.ServiceKindTypical
	}
}

func (b *Builder) serviceProxyVal(v *ServiceProxy) *structs.ConnectProxyConfig {
	if v == nil {
		return nil
	}

	return &structs.ConnectProxyConfig{
		DestinationServiceName: b.stringVal(v.DestinationServiceName),
		DestinationServiceID:   b.stringVal(v.DestinationServiceID),
		LocalServiceAddress:    b.stringVal(v.LocalServiceAddress),
		LocalServicePort:       b.intVal(v.LocalServicePort),
		Config:                 v.Config,
		Upstreams:              b.upstreamsVal(v.Upstreams),
		MeshGateway:            b.meshGatewayConfVal(v.MeshGateway),
	}
}

func (b *Builder) upstreamsVal(v []Upstream) structs.Upstreams {
	ups := make(structs.Upstreams, len(v))
	for i, u := range v {
		ups[i] = structs.Upstream{
			DestinationType:      b.stringVal(u.DestinationType),
			DestinationNamespace: b.stringVal(u.DestinationNamespace),
			DestinationName:      b.stringVal(u.DestinationName),
			Datacenter:           b.stringVal(u.Datacenter),
			LocalBindAddress:     b.stringVal(u.LocalBindAddress),
			LocalBindPort:        b.intVal(u.LocalBindPort),
			Config:               u.Config,
			MeshGateway:          b.meshGatewayConfVal(u.MeshGateway),
		}
		if ups[i].DestinationType == "" {
			ups[i].DestinationType = structs.UpstreamDestTypeService
		}
	}
	return ups
}

func (b *Builder) meshGatewayConfVal(mgConf *MeshGatewayConfig) structs.MeshGatewayConfig {
	cfg := structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeDefault}
	if mgConf == nil || mgConf.Mode == nil {
		// return defaults
		return cfg
	}

	mode, err := structs.ValidateMeshGatewayMode(*mgConf.Mode)
	if err != nil {
		b.err = multierror.Append(b.err, err)
		return cfg
	}

	cfg.Mode = mode
	return cfg
}

func (b *Builder) serviceConnectVal(v *ServiceConnect) *structs.ServiceConnect {
	if v == nil {
		return nil
	}

	sidecar := b.serviceVal(v.SidecarService)
	if sidecar != nil {
		// Sanity checks
		if sidecar.ID != "" {
			b.err = multierror.Append(b.err, fmt.Errorf("sidecar_service can't specify an ID"))
			sidecar.ID = ""
		}
		if sidecar.Connect != nil {
			if sidecar.Connect.SidecarService != nil {
				b.err = multierror.Append(b.err, fmt.Errorf("sidecar_service can't have a nested sidecar_service"))
				sidecar.Connect.SidecarService = nil
			}
		}
	}

	return &structs.ServiceConnect{
		Native:         b.boolVal(v.Native),
		SidecarService: sidecar,
	}
}

func (b *Builder) boolValWithDefault(v *bool, defaultVal bool) bool {
	if v == nil {
		return defaultVal
	}

	return *v
}

func (b *Builder) boolVal(v *bool) bool {
	return b.boolValWithDefault(v, false)
}

func (b *Builder) durationValWithDefault(name string, v *string, defaultVal time.Duration) (d time.Duration) {
	if v == nil {
		return defaultVal
	}
	d, err := time.ParseDuration(*v)
	if err != nil {
		b.err = multierror.Append(fmt.Errorf("%s: invalid duration: %q: %s", name, *v, err))
	}
	return d
}

func (b *Builder) durationVal(name string, v *string) (d time.Duration) {
	return b.durationValWithDefault(name, v, 0)
}

func (b *Builder) intValWithDefault(v *int, defaultVal int) int {
	if v == nil {
		return defaultVal
	}
	return *v
}

func (b *Builder) intVal(v *int) int {
	return b.intValWithDefault(v, 0)
}

func (b *Builder) uint64ValWithDefault(v *uint64, defaultVal uint64) uint64 {
	if v == nil {
		return defaultVal
	}
	return *v
}

func (b *Builder) uint64Val(v *uint64) uint64 {
	return b.uint64ValWithDefault(v, 0)
}

func (b *Builder) portVal(name string, v *int) int {
	if v == nil || *v <= 0 {
		return -1
	}
	if *v > 65535 {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: invalid port: %d", name, *v))
	}
	return *v
}

func (b *Builder) stringValWithDefault(v *string, defaultVal string) string {
	if v == nil {
		return defaultVal
	}
	return *v
}

func (b *Builder) stringVal(v *string) string {
	return b.stringValWithDefault(v, "")
}

func (b *Builder) float64Val(v *float64) float64 {
	if v == nil {
		return 0
	}

	return *v
}

func (b *Builder) cidrsVal(name string, v []string) (nets []*net.IPNet) {
	if v == nil {
		return
	}

	for _, p := range v {
		_, net, err := net.ParseCIDR(strings.TrimSpace(p))
		if err != nil {
			b.err = multierror.Append(b.err, fmt.Errorf("%s: invalid cidr: %s", name, p))
		}
		nets = append(nets, net)
	}

	return
}

func (b *Builder) tlsCipherSuites(name string, v *string) []uint16 {
	if v == nil {
		return nil
	}

	var a []uint16
	a, err := tlsutil.ParseCiphers(*v)
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: invalid tls cipher suites: %s", name, err))
	}
	return a
}

func (b *Builder) nodeName(v *string) string {
	nodeName := b.stringVal(v)
	if nodeName == "" {
		fn := b.Hostname
		if fn == nil {
			fn = os.Hostname
		}
		name, err := fn()
		if err != nil {
			b.err = multierror.Append(b.err, fmt.Errorf("node_name: %s", err))
			return ""
		}
		nodeName = name
	}
	return strings.TrimSpace(nodeName)
}

// expandAddrs expands the go-sockaddr template in s and returns the
// result as a list of *net.IPAddr and *net.UnixAddr.
func (b *Builder) expandAddrs(name string, s *string) []net.Addr {
	if s == nil || *s == "" {
		return nil
	}

	x, err := template.Parse(*s)
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: error parsing %q: %s", name, *s, err))
		return nil
	}

	var addrs []net.Addr
	for _, a := range strings.Fields(x) {
		switch {
		case strings.HasPrefix(a, "unix://"):
			addrs = append(addrs, &net.UnixAddr{Name: a[len("unix://"):], Net: "unix"})
		default:
			// net.ParseIP does not like '[::]'
			ip := net.ParseIP(a)
			if a == "[::]" {
				ip = net.ParseIP("::")
			}
			if ip == nil {
				b.err = multierror.Append(b.err, fmt.Errorf("%s: invalid ip address: %s", name, a))
				return nil
			}
			addrs = append(addrs, &net.IPAddr{IP: ip})
		}
	}

	return addrs
}

// expandOptionalAddrs expands the go-sockaddr template in s and returns the
// result as a list of strings. If s does not contain a go-sockaddr template,
// the result list will contain the input string as a single element with no
// error set. In contrast to expandAddrs, expandOptionalAddrs does not validate
// if the result contains valid addresses and returns a list of strings.
// However, if the expansion of the go-sockaddr template fails an error is set.
func (b *Builder) expandOptionalAddrs(name string, s *string) []string {
	if s == nil || *s == "" {
		return nil
	}

	x, err := template.Parse(*s)
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: error parsing %q: %s", name, *s, err))
		return nil
	}

	if x != *s {
		// A template has been expanded, split the results from go-sockaddr
		return strings.Fields(x)
	} else {
		// No template has been expanded, pass through the input
		return []string{*s}
	}
}

func (b *Builder) expandAllOptionalAddrs(name string, addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		expanded := b.expandOptionalAddrs(name, &a)
		if expanded != nil {
			out = append(out, expanded...)
		}
	}
	return out
}

// expandIPs expands the go-sockaddr template in s and returns a list of
// *net.IPAddr. If one of the expanded addresses is a unix socket
// address an error is set and nil is returned.
func (b *Builder) expandIPs(name string, s *string) []*net.IPAddr {
	if s == nil || *s == "" {
		return nil
	}

	addrs := b.expandAddrs(name, s)
	var x []*net.IPAddr
	for _, addr := range addrs {
		switch a := addr.(type) {
		case *net.IPAddr:
			x = append(x, a)
		case *net.UnixAddr:
			b.err = multierror.Append(b.err, fmt.Errorf("%s cannot be a unix socket", name))
			return nil
		default:
			b.err = multierror.Append(b.err, fmt.Errorf("%s has invalid address type %T", name, a))
			return nil
		}
	}
	return x
}

// expandFirstAddr expands the go-sockaddr template in s and returns the
// first address which is either a *net.IPAddr or a *net.UnixAddr. If
// the template expands to multiple addresses an error is set and nil
// is returned.
func (b *Builder) expandFirstAddr(name string, s *string) net.Addr {
	if s == nil || *s == "" {
		return nil
	}

	addrs := b.expandAddrs(name, s)
	if len(addrs) == 0 {
		return nil
	}
	if len(addrs) > 1 {
		var x []string
		for _, a := range addrs {
			x = append(x, a.String())
		}
		b.err = multierror.Append(b.err, fmt.Errorf("%s: multiple addresses found: %s", name, strings.Join(x, " ")))
		return nil
	}
	return addrs[0]
}

// expandFirstIP expands the go-sockaddr template in s and returns the
// first address if it is not a unix socket address. If the template
// expands to multiple addresses an error is set and nil is returned.
func (b *Builder) expandFirstIP(name string, s *string) *net.IPAddr {
	if s == nil || *s == "" {
		return nil
	}

	addr := b.expandFirstAddr(name, s)
	if addr == nil {
		return nil
	}
	switch a := addr.(type) {
	case *net.IPAddr:
		return a
	case *net.UnixAddr:
		b.err = multierror.Append(b.err, fmt.Errorf("%s cannot be a unix socket", name))
		return nil
	default:
		b.err = multierror.Append(b.err, fmt.Errorf("%s has invalid address type %T", name, a))
		return nil
	}
}

func (b *Builder) makeIPAddr(pri *net.IPAddr, sec *net.IPAddr) *net.IPAddr {
	if pri != nil {
		return pri
	}
	return sec
}

func (b *Builder) makeTCPAddr(pri *net.IPAddr, sec net.Addr, port int) *net.TCPAddr {
	if pri == nil && reflect.ValueOf(sec).IsNil() || port <= 0 {
		return nil
	}
	addr := pri
	if addr == nil {
		switch a := sec.(type) {
		case *net.IPAddr:
			addr = a
		case *net.TCPAddr:
			addr = &net.IPAddr{IP: a.IP}
		default:
			panic(fmt.Sprintf("makeTCPAddr requires a net.IPAddr or a net.TCPAddr. Got %T", a))
		}
	}
	return &net.TCPAddr{IP: addr.IP, Port: port}
}

// makeAddr creates an *net.TCPAddr or a *net.UnixAddr from either the
// primary or secondary address and the given port. If the port is <= 0
// then the address is considered to be disabled and nil is returned.
func (b *Builder) makeAddr(pri, sec net.Addr, port int) net.Addr {
	if reflect.ValueOf(pri).IsNil() && reflect.ValueOf(sec).IsNil() || port <= 0 {
		return nil
	}
	addr := pri
	if addr == nil {
		addr = sec
	}
	switch a := addr.(type) {
	case *net.IPAddr:
		return &net.TCPAddr{IP: a.IP, Port: port}
	case *net.UnixAddr:
		return a
	default:
		panic(fmt.Sprintf("invalid address type %T", a))
	}
}

// makeAddrs creates a list of *net.TCPAddr or *net.UnixAddr entries
// from either the primary or secondary addresses and the given port.
// If the port is <= 0 then the address is considered to be disabled
// and nil is returned.
func (b *Builder) makeAddrs(pri []net.Addr, sec []*net.IPAddr, port int) []net.Addr {
	if len(pri) == 0 && len(sec) == 0 || port <= 0 {
		return nil
	}
	addrs := pri
	if len(addrs) == 0 {
		addrs = []net.Addr{}
		for _, a := range sec {
			addrs = append(addrs, a)
		}
	}
	var x []net.Addr
	for _, a := range addrs {
		x = append(x, b.makeAddr(a, nil, port))
	}
	return x
}

// isUnixAddr returns true when the given address is a unix socket address type.
func (b *Builder) isUnixAddr(a net.Addr) bool {
	_, ok := a.(*net.UnixAddr)
	return a != nil && ok
}

// decodeBytes returns the encryption key decoded.
func decodeBytes(key string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(key)
}

func isIPAddr(a net.Addr) bool {
	_, ok := a.(*net.IPAddr)
	return ok
}

func isUnixAddr(a net.Addr) bool {
	_, ok := a.(*net.UnixAddr)
	return ok
}

// isValidAltDomain returns true if the given domain is not prefixed
// by keywords used when dispatching DNS requests
func isValidAltDomain(domain, datacenter string) bool {
	reAltDomain := regexp.MustCompile(
		fmt.Sprintf(
			"^(service|connect|node|query|addr|%s)\\.(%s\\.)?",
			datacenter, datacenter,
		),
	)
	return !reAltDomain.MatchString(domain)
}

// UIPathBuilder checks to see if there was a path set
// If so, adds beginning and trailing slashes to UI path
func UIPathBuilder(UIContentString string) string {
	if UIContentString != "" {
		var fmtedPath string
		fmtedPath = strings.Trim(UIContentString, "/")
		fmtedPath = "/" + fmtedPath + "/"
		return fmtedPath

	}
	return "/ui/"
}
