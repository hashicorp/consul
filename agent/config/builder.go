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

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/ipaddr"
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
// concatenating the two slices.
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
	b.Tail = append(b.Tail, NonUserSource(), DefaultConsulSource(), DefaultVersionSource())
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
		// do not recurse into sub dirs
		if fi.IsDir() {
			continue
		}

		src, err := b.ReadFile(filepath.Join(path, fi.Name()))
		if err != nil {
			return nil, err
		}
		sources = append(sources, src)
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
		src.Format = FormatFrom(src.Name)
		if configFormat != "" {
			src.Format = configFormat
		} else {
			// If they haven't forced things to a specific format,
			// then skip anything we don't understand, which is the
			// behavior before we added the -config-format option.
			switch src.Format {
			case "json", "hcl":
				// OK
			default:
				// SKIP
				continue
			}
		}
		if src.Format == "" {
			return RuntimeConfig{}, fmt.Errorf(`config: Missing or invalid file extension for %q. Please use ".json" or ".hcl".`, src.Name)
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
	serfPortLAN := b.portVal("ports.serf_lan", c.Ports.SerfLAN)
	serfPortWAN := b.portVal("ports.serf_wan", c.Ports.SerfWAN)

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
			return RuntimeConfig{}, fmt.Errorf("Multiple %s addresses found. Please configure one", addrtyp)
		}
		advertiseAddr = advertiseAddrs[0]
	}

	// derive other bind addresses from the bindAddr
	rpcBindAddr := b.makeTCPAddr(bindAddr, nil, serverPort)
	serfBindAddrLAN := b.makeTCPAddr(b.expandFirstIP("serf_lan", c.SerfBindAddrLAN), bindAddr, serfPortLAN)
	serfBindAddrWAN := b.makeTCPAddr(b.expandFirstIP("serf_wan", c.SerfBindAddrWAN), bindAddr, serfPortWAN)

	// derive other advertise addresses from the advertise address
	advertiseAddrLAN := b.makeIPAddr(b.expandFirstIP("advertise_addr", c.AdvertiseAddrLAN), advertiseAddr)
	advertiseAddrWAN := b.makeIPAddr(b.expandFirstIP("advertise_addr_wan", c.AdvertiseAddrWAN), advertiseAddrLAN)
	rpcAdvertiseAddr := &net.TCPAddr{IP: advertiseAddrLAN.IP, Port: serverPort}
	serfAdvertiseAddrLAN := &net.TCPAddr{IP: advertiseAddrLAN.IP, Port: serfPortLAN}
	serfAdvertiseAddrWAN := &net.TCPAddr{IP: advertiseAddrWAN.IP, Port: serfPortWAN}

	// determine client addresses
	clientAddrs := b.expandIPs("client_addr", c.ClientAddr)
	dnsAddrs := b.makeAddrs(b.expandAddrs("addresses.dns", c.Addresses.DNS), clientAddrs, dnsPort)
	httpAddrs := b.makeAddrs(b.expandAddrs("addresses.http", c.Addresses.HTTP), clientAddrs, httpPort)
	httpsAddrs := b.makeAddrs(b.expandAddrs("addresses.https", c.Addresses.HTTPS), clientAddrs, httpsPort)

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

	// Add a filter rule if needed for enabling the deprecated metric names
	enableDeprecatedNames := b.boolVal(c.Telemetry.EnableDeprecatedNames)
	if enableDeprecatedNames {
		telemetryAllowedPrefixes = append(telemetryAllowedPrefixes, "consul.consul")
	} else {
		telemetryBlockedPrefixes = append(telemetryBlockedPrefixes, "consul.consul")
	}

	// raft performance scaling
	performanceRaftMultiplier := b.intVal(c.Performance.RaftMultiplier)
	if performanceRaftMultiplier < 1 || uint(performanceRaftMultiplier) > consul.MaxRaftMultiplier {
		return RuntimeConfig{}, fmt.Errorf("performance.raft_multiplier cannot be %d. Must be between 1 and %d", performanceRaftMultiplier, consul.MaxRaftMultiplier)
	}
	consulRaftElectionTimeout := b.durationVal("consul.raft.election_timeout", c.Consul.Raft.ElectionTimeout) * time.Duration(performanceRaftMultiplier)
	consulRaftHeartbeatTimeout := b.durationVal("consul.raft.heartbeat_timeout", c.Consul.Raft.HeartbeatTimeout) * time.Duration(performanceRaftMultiplier)
	consulRaftLeaderLeaseTimeout := b.durationVal("consul.raft.leader_lease_timeout", c.Consul.Raft.LeaderLeaseTimeout) * time.Duration(performanceRaftMultiplier)

	// ----------------------------------------------------------------
	// build runtime config
	//
	rt = RuntimeConfig{
		// non-user configurable values
		ACLDisabledTTL:             b.durationVal("acl_disabled_ttl", c.ACLDisabledTTL),
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
		ConsulSerfLANGossipInterval:      b.durationVal("consul.serf_lan.gossip_interval", c.Consul.SerfLAN.Memberlist.GossipInterval),
		ConsulSerfLANProbeInterval:       b.durationVal("consul.serf_lan.probe_interval", c.Consul.SerfLAN.Memberlist.ProbeInterval),
		ConsulSerfLANProbeTimeout:        b.durationVal("consul.serf_lan.probe_timeout", c.Consul.SerfLAN.Memberlist.ProbeTimeout),
		ConsulSerfLANSuspicionMult:       b.intVal(c.Consul.SerfLAN.Memberlist.SuspicionMult),
		ConsulSerfWANGossipInterval:      b.durationVal("consul.serf_wan.gossip_interval", c.Consul.SerfWAN.Memberlist.GossipInterval),
		ConsulSerfWANProbeInterval:       b.durationVal("consul.serf_wan.probe_interval", c.Consul.SerfWAN.Memberlist.ProbeInterval),
		ConsulSerfWANProbeTimeout:        b.durationVal("consul.serf_wan.probe_timeout", c.Consul.SerfWAN.Memberlist.ProbeTimeout),
		ConsulSerfWANSuspicionMult:       b.intVal(c.Consul.SerfWAN.Memberlist.SuspicionMult),
		ConsulServerHealthInterval:       b.durationVal("consul.server.health_interval", c.Consul.Server.HealthInterval),

		// ACL
		ACLAgentMasterToken:    b.stringVal(c.ACLAgentMasterToken),
		ACLAgentToken:          b.stringVal(c.ACLAgentToken),
		ACLDatacenter:          strings.ToLower(b.stringVal(c.ACLDatacenter)),
		ACLDefaultPolicy:       b.stringVal(c.ACLDefaultPolicy),
		ACLDownPolicy:          b.stringVal(c.ACLDownPolicy),
		ACLEnforceVersion8:     b.boolVal(c.ACLEnforceVersion8),
		ACLEnableKeyListPolicy: b.boolVal(c.ACLEnableKeyListPolicy),
		ACLMasterToken:         b.stringVal(c.ACLMasterToken),
		ACLReplicationToken:    b.stringVal(c.ACLReplicationToken),
		ACLTTL:                 b.durationVal("acl_ttl", c.ACLTTL),
		ACLToken:               b.stringVal(c.ACLToken),
		EnableACLReplication:   b.boolVal(c.EnableACLReplication),

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
		DNSDisableCompression: b.boolVal(c.DNS.DisableCompression),
		DNSDomain:             b.stringVal(c.DNSDomain),
		DNSEnableTruncate:     b.boolVal(c.DNS.EnableTruncate),
		DNSMaxStale:           b.durationVal("dns_config.max_stale", c.DNS.MaxStale),
		DNSNodeTTL:            b.durationVal("dns_config.node_ttl", c.DNS.NodeTTL),
		DNSOnlyPassing:        b.boolVal(c.DNS.OnlyPassing),
		DNSPort:               dnsPort,
		DNSRecursorTimeout:    b.durationVal("recursor_timeout", c.DNS.RecursorTimeout),
		DNSRecursors:          dnsRecursors,
		DNSServiceTTL:         dnsServiceTTL,
		DNSUDPAnswerLimit:     b.intVal(c.DNS.UDPAnswerLimit),

		// HTTP
		HTTPPort:            httpPort,
		HTTPSPort:           httpsPort,
		HTTPAddrs:           httpAddrs,
		HTTPSAddrs:          httpsAddrs,
		HTTPBlockEndpoints:  c.HTTPConfig.BlockEndpoints,
		HTTPResponseHeaders: c.HTTPConfig.ResponseHeaders,

		// Telemetry
		TelemetryCirconusAPIApp:                     b.stringVal(c.Telemetry.CirconusAPIApp),
		TelemetryCirconusAPIToken:                   b.stringVal(c.Telemetry.CirconusAPIToken),
		TelemetryCirconusAPIURL:                     b.stringVal(c.Telemetry.CirconusAPIURL),
		TelemetryCirconusBrokerID:                   b.stringVal(c.Telemetry.CirconusBrokerID),
		TelemetryCirconusBrokerSelectTag:            b.stringVal(c.Telemetry.CirconusBrokerSelectTag),
		TelemetryCirconusCheckDisplayName:           b.stringVal(c.Telemetry.CirconusCheckDisplayName),
		TelemetryCirconusCheckForceMetricActivation: b.stringVal(c.Telemetry.CirconusCheckForceMetricActivation),
		TelemetryCirconusCheckID:                    b.stringVal(c.Telemetry.CirconusCheckID),
		TelemetryCirconusCheckInstanceID:            b.stringVal(c.Telemetry.CirconusCheckInstanceID),
		TelemetryCirconusCheckSearchTag:             b.stringVal(c.Telemetry.CirconusCheckSearchTag),
		TelemetryCirconusCheckTags:                  b.stringVal(c.Telemetry.CirconusCheckTags),
		TelemetryCirconusSubmissionInterval:         b.stringVal(c.Telemetry.CirconusSubmissionInterval),
		TelemetryCirconusSubmissionURL:              b.stringVal(c.Telemetry.CirconusSubmissionURL),
		TelemetryDisableHostname:                    b.boolVal(c.Telemetry.DisableHostname),
		TelemetryDogstatsdAddr:                      b.stringVal(c.Telemetry.DogstatsdAddr),
		TelemetryDogstatsdTags:                      c.Telemetry.DogstatsdTags,
		TelemetryFilterDefault:                      b.boolVal(c.Telemetry.FilterDefault),
		TelemetryAllowedPrefixes:                    telemetryAllowedPrefixes,
		TelemetryBlockedPrefixes:                    telemetryBlockedPrefixes,
		TelemetryMetricsPrefix:                      b.stringVal(c.Telemetry.MetricsPrefix),
		TelemetryStatsdAddr:                         b.stringVal(c.Telemetry.StatsdAddr),
		TelemetryStatsiteAddr:                       b.stringVal(c.Telemetry.StatsiteAddr),

		// Agent
		AdvertiseAddrLAN:            advertiseAddrLAN,
		AdvertiseAddrWAN:            advertiseAddrWAN,
		BindAddr:                    bindAddr,
		Bootstrap:                   b.boolVal(c.Bootstrap),
		BootstrapExpect:             b.intVal(c.BootstrapExpect),
		CAFile:                      b.stringVal(c.CAFile),
		CAPath:                      b.stringVal(c.CAPath),
		CertFile:                    b.stringVal(c.CertFile),
		CheckUpdateInterval:         b.durationVal("check_update_interval", c.CheckUpdateInterval),
		Checks:                      checks,
		ClientAddrs:                 clientAddrs,
		DataDir:                     b.stringVal(c.DataDir),
		Datacenter:                  strings.ToLower(b.stringVal(c.Datacenter)),
		DevMode:                     b.boolVal(b.Flags.DevMode),
		DisableAnonymousSignature:   b.boolVal(c.DisableAnonymousSignature),
		DisableCoordinates:          b.boolVal(c.DisableCoordinates),
		DisableHostNodeID:           b.boolVal(c.DisableHostNodeID),
		DisableKeyringFile:          b.boolVal(c.DisableKeyringFile),
		DisableRemoteExec:           b.boolVal(c.DisableRemoteExec),
		DisableUpdateCheck:          b.boolVal(c.DisableUpdateCheck),
		DiscardCheckOutput:          b.boolVal(c.DiscardCheckOutput),
		EnableAgentTLSForChecks:     b.boolVal(c.EnableAgentTLSForChecks),
		EnableDebug:                 b.boolVal(c.EnableDebug),
		EnableScriptChecks:          b.boolVal(c.EnableScriptChecks),
		EnableSyslog:                b.boolVal(c.EnableSyslog),
		EnableUI:                    b.boolVal(c.UI),
		EncryptKey:                  b.stringVal(c.EncryptKey),
		EncryptVerifyIncoming:       b.boolVal(c.EncryptVerifyIncoming),
		EncryptVerifyOutgoing:       b.boolVal(c.EncryptVerifyOutgoing),
		KeyFile:                     b.stringVal(c.KeyFile),
		LeaveDrainTime:              b.durationVal("performance.leave_drain_time", c.Performance.LeaveDrainTime),
		LeaveOnTerm:                 leaveOnTerm,
		LogLevel:                    b.stringVal(c.LogLevel),
		NodeID:                      types.NodeID(b.stringVal(c.NodeID)),
		NodeMeta:                    c.NodeMeta,
		NodeName:                    b.nodeName(c.NodeName),
		NonVotingServer:             b.boolVal(c.NonVotingServer),
		PidFile:                     b.stringVal(c.PidFile),
		RPCAdvertiseAddr:            rpcAdvertiseAddr,
		RPCBindAddr:                 rpcBindAddr,
		RPCHoldTimeout:              b.durationVal("performance.rpc_hold_timeout", c.Performance.RPCHoldTimeout),
		RPCMaxBurst:                 b.intVal(c.Limits.RPCMaxBurst),
		RPCProtocol:                 b.intVal(c.RPCProtocol),
		RPCRateLimit:                rate.Limit(b.float64Val(c.Limits.RPCRate)),
		RaftProtocol:                b.intVal(c.RaftProtocol),
		ReconnectTimeoutLAN:         b.durationVal("reconnect_timeout", c.ReconnectTimeoutLAN),
		ReconnectTimeoutWAN:         b.durationVal("reconnect_timeout_wan", c.ReconnectTimeoutWAN),
		RejoinAfterLeave:            b.boolVal(c.RejoinAfterLeave),
		RetryJoinIntervalLAN:        b.durationVal("retry_interval", c.RetryJoinIntervalLAN),
		RetryJoinIntervalWAN:        b.durationVal("retry_interval_wan", c.RetryJoinIntervalWAN),
		RetryJoinLAN:                c.RetryJoinLAN,
		RetryJoinMaxAttemptsLAN:     b.intVal(c.RetryJoinMaxAttemptsLAN),
		RetryJoinMaxAttemptsWAN:     b.intVal(c.RetryJoinMaxAttemptsWAN),
		RetryJoinWAN:                c.RetryJoinWAN,
		SegmentName:                 b.stringVal(c.SegmentName),
		Segments:                    segments,
		SerfAdvertiseAddrLAN:        serfAdvertiseAddrLAN,
		SerfAdvertiseAddrWAN:        serfAdvertiseAddrWAN,
		SerfBindAddrLAN:             serfBindAddrLAN,
		SerfBindAddrWAN:             serfBindAddrWAN,
		SerfPortLAN:                 serfPortLAN,
		SerfPortWAN:                 serfPortWAN,
		ServerMode:                  b.boolVal(c.ServerMode),
		ServerName:                  b.stringVal(c.ServerName),
		ServerPort:                  serverPort,
		Services:                    services,
		SessionTTLMin:               b.durationVal("session_ttl_min", c.SessionTTLMin),
		SkipLeaveOnInt:              skipLeaveOnInt,
		StartJoinAddrsLAN:           c.StartJoinAddrsLAN,
		StartJoinAddrsWAN:           c.StartJoinAddrsWAN,
		SyslogFacility:              b.stringVal(c.SyslogFacility),
		TLSCipherSuites:             b.tlsCipherSuites("tls_cipher_suites", c.TLSCipherSuites),
		TLSMinVersion:               b.stringVal(c.TLSMinVersion),
		TLSPreferServerCipherSuites: b.boolVal(c.TLSPreferServerCipherSuites),
		TaggedAddresses:             c.TaggedAddresses,
		TranslateWANAddrs:           b.boolVal(c.TranslateWANAddrs),
		UIDir:                       b.stringVal(c.UIDir),
		UnixSocketGroup:             b.stringVal(c.UnixSocket.Group),
		UnixSocketMode:              b.stringVal(c.UnixSocket.Mode),
		UnixSocketUser:              b.stringVal(c.UnixSocket.User),
		VerifyIncoming:              b.boolVal(c.VerifyIncoming),
		VerifyIncomingHTTPS:         b.boolVal(c.VerifyIncomingHTTPS),
		VerifyIncomingRPC:           b.boolVal(c.VerifyIncomingRPC),
		VerifyOutgoing:              b.boolVal(c.VerifyOutgoing),
		VerifyServerHostname:        b.boolVal(c.VerifyServerHostname),
		Watches:                     c.Watches,
	}

	if rt.BootstrapExpect == 1 {
		rt.Bootstrap = true
		rt.BootstrapExpect = 0
		b.warn(`BootstrapExpect is set to 1; this is the same as Bootstrap mode.`)
	}

	if rt.ACLReplicationToken != "" {
		rt.EnableACLReplication = true
	}

	return rt, nil
}

// Validate performs semantical validation of the runtime configuration.
func (b *Builder) Validate(rt RuntimeConfig) error {
	// reDatacenter defines a regexp for a valid datacenter name
	var reDatacenter = regexp.MustCompile("^[a-z0-9_-]+$")

	// ----------------------------------------------------------------
	// check required params we cannot recover from first
	//

	if rt.Datacenter == "" {
		return fmt.Errorf("datacenter cannot be empty")
	}
	if !reDatacenter.MatchString(rt.Datacenter) {
		return fmt.Errorf("datacenter cannot be %q. Please use only [a-z0-9-_].", rt.Datacenter)
	}
	if rt.DataDir == "" && !rt.DevMode {
		return fmt.Errorf("data_dir cannot be empty")
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
	if rt.AEInterval <= 0 {
		return fmt.Errorf("ae_interval cannot be %s. Must be positive", rt.AEInterval)
	}
	if rt.AutopilotMaxTrailingLogs < 0 {
		return fmt.Errorf("autopilot.max_trailing_logs cannot be %d. Must be greater than or equal to zero", rt.AutopilotMaxTrailingLogs)
	}
	if rt.ACLDatacenter != "" && !reDatacenter.MatchString(rt.ACLDatacenter) {
		return fmt.Errorf("acl_datacenter cannot be %q. Please use only [a-z0-9-_].", rt.ACLDatacenter)
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
	if err := addrUnique(inuse, "Serf Advertise WAN", rt.SerfAdvertiseAddrWAN); err != nil {
		return err
	}
	if b.err != nil {
		return b.err
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
		ID:                id,
		Name:              b.stringVal(v.Name),
		Notes:             b.stringVal(v.Notes),
		ServiceID:         b.stringVal(v.ServiceID),
		Token:             b.stringVal(v.Token),
		Status:            b.stringVal(v.Status),
		Script:            b.stringVal(v.Script),
		ScriptArgs:        v.ScriptArgs,
		HTTP:              b.stringVal(v.HTTP),
		Header:            v.Header,
		Method:            b.stringVal(v.Method),
		TCP:               b.stringVal(v.TCP),
		Interval:          b.durationVal(fmt.Sprintf("check[%s].interval", id), v.Interval),
		DockerContainerID: b.stringVal(v.DockerContainerID),
		Shell:             b.stringVal(v.Shell),
		TLSSkipVerify:     b.boolVal(v.TLSSkipVerify),
		Timeout:           b.durationVal(fmt.Sprintf("check[%s].timeout", id), v.Timeout),
		TTL:               b.durationVal(fmt.Sprintf("check[%s].ttl", id), v.TTL),
		DeregisterCriticalServiceAfter: b.durationVal(fmt.Sprintf("check[%s].deregister_critical_service_after", id), v.DeregisterCriticalServiceAfter),
	}
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

	return &structs.ServiceDefinition{
		ID:                b.stringVal(v.ID),
		Name:              b.stringVal(v.Name),
		Tags:              v.Tags,
		Address:           b.stringVal(v.Address),
		Port:              b.intVal(v.Port),
		Token:             b.stringVal(v.Token),
		EnableTagOverride: b.boolVal(v.EnableTagOverride),
		Checks:            checks,
	}
}

func (b *Builder) boolVal(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func (b *Builder) durationVal(name string, v *string) (d time.Duration) {
	if v == nil {
		return 0
	}
	d, err := time.ParseDuration(*v)
	if err != nil {
		b.err = multierror.Append(fmt.Errorf("%s: invalid duration: %q: %s", name, *v, err))
	}
	return d
}

func (b *Builder) intVal(v *int) int {
	if v == nil {
		return 0
	}
	return *v
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

func (b *Builder) stringVal(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func (b *Builder) float64Val(v *float64) float64 {
	if v == nil {
		return 0
	}

	return *v
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
		b.err = multierror.Append(b.err, fmt.Errorf("%s: error parsing %q: %s", name, s, err))
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

// expandFirstIP exapnds the go-sockaddr template in s and returns the
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
