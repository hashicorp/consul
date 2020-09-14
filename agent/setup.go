package agent

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	autoconf "github.com/hashicorp/consul/agent/auto-config"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc/grpclog"
)

// TODO: BaseDeps should be renamed in the future once more of Agent.Start
// has been moved out in front of Agent.New, and we can better see the setup
// dependencies.
type BaseDeps struct {
	consul.Deps // TODO: un-embed

	RuntimeConfig  *config.RuntimeConfig
	MetricsHandler MetricsHandler
	AutoConfig     *autoconf.AutoConfig // TODO: use an interface
	Cache          *cache.Cache
}

// MetricsHandler provides an http.Handler for displaying metrics.
type MetricsHandler interface {
	DisplayMetrics(resp http.ResponseWriter, req *http.Request) (interface{}, error)
}

type ConfigLoader func(source config.Source) (cfg *config.RuntimeConfig, warnings []string, err error)

func NewBaseDeps(configLoader ConfigLoader, logOut io.Writer) (BaseDeps, error) {
	d := BaseDeps{}
	cfg, warnings, err := configLoader(nil)
	if err != nil {
		return d, err
	}

	logConf := cfg.Logging
	logConf.Name = logging.Agent
	d.Logger, err = logging.Setup(logConf, logOut)
	if err != nil {
		return d, err
	}
	grpclog.SetLoggerV2(logging.NewGRPCLogger(cfg.Logging.LogLevel, d.Logger))

	for _, w := range warnings {
		d.Logger.Warn(w)
	}

	cfg.NodeID, err = newNodeIDFromConfig(cfg, d.Logger)
	if err != nil {
		return d, fmt.Errorf("failed to setup node ID: %w", err)
	}

	d.MetricsHandler, err = lib.InitTelemetry(cfg.Telemetry)
	if err != nil {
		return d, fmt.Errorf("failed to initialize telemetry: %w", err)
	}

	d.TLSConfigurator, err = tlsutil.NewConfigurator(cfg.ToTLSUtilConfig(), d.Logger)
	if err != nil {
		return d, err
	}

	d.RuntimeConfig = cfg
	d.Tokens = new(token.Store)

	// cache-types are not registered yet, but they won't be used until the components are started.
	d.Cache = cache.New(cfg.Cache)
	d.ConnPool = newConnPool(cfg, d.Logger, d.TLSConfigurator)

	d.Router = router.NewRouter(d.Logger, cfg.Datacenter, fmt.Sprintf("%s.%s", cfg.NodeName, cfg.Datacenter))

	acConf := autoconf.Config{
		DirectRPC:       d.ConnPool,
		Logger:          d.Logger,
		Loader:          configLoader,
		ServerProvider:  d.Router,
		TLSConfigurator: d.TLSConfigurator,
		Cache:           d.Cache,
		Tokens:          d.Tokens,
	}
	d.AutoConfig, err = autoconf.New(acConf)
	if err != nil {
		return d, err
	}

	return d, nil
}

func newConnPool(config *config.RuntimeConfig, logger hclog.Logger, tls *tlsutil.Configurator) *pool.ConnPool {
	var rpcSrcAddr *net.TCPAddr
	if !ipaddr.IsAny(config.RPCBindAddr) {
		rpcSrcAddr = &net.TCPAddr{IP: config.RPCBindAddr.IP}
	}

	pool := &pool.ConnPool{
		Server:          config.ServerMode,
		SrcAddr:         rpcSrcAddr,
		Logger:          logger.StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true}),
		TLSConfigurator: tls,
		Datacenter:      config.Datacenter,
	}
	if config.ServerMode {
		pool.MaxTime = 2 * time.Minute
		pool.MaxStreams = 64
	} else {
		// MaxTime controls how long we keep an idle connection open to a server.
		// 127s was chosen as the first prime above 120s
		// (arbitrarily chose to use a prime) with the intent of reusing
		// connections who are used by once-a-minute cron(8) jobs *and* who
		// use a 60s jitter window (e.g. in vixie cron job execution can
		// drift by up to 59s per job, or 119s for a once-a-minute cron job).
		pool.MaxTime = 127 * time.Second
		pool.MaxStreams = 32
	}
	return pool
}
