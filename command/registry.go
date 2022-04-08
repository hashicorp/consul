package command

import (
	"github.com/hashicorp/consul/command/acl"
	aclagent "github.com/hashicorp/consul/command/acl/agenttokens"
	aclam "github.com/hashicorp/consul/command/acl/authmethod"
	aclamcreate "github.com/hashicorp/consul/command/acl/authmethod/create"
	aclamdelete "github.com/hashicorp/consul/command/acl/authmethod/delete"
	aclamlist "github.com/hashicorp/consul/command/acl/authmethod/list"
	aclamread "github.com/hashicorp/consul/command/acl/authmethod/read"
	aclamupdate "github.com/hashicorp/consul/command/acl/authmethod/update"
	aclbr "github.com/hashicorp/consul/command/acl/bindingrule"
	aclbrcreate "github.com/hashicorp/consul/command/acl/bindingrule/create"
	aclbrdelete "github.com/hashicorp/consul/command/acl/bindingrule/delete"
	aclbrlist "github.com/hashicorp/consul/command/acl/bindingrule/list"
	aclbrread "github.com/hashicorp/consul/command/acl/bindingrule/read"
	aclbrupdate "github.com/hashicorp/consul/command/acl/bindingrule/update"
	aclbootstrap "github.com/hashicorp/consul/command/acl/bootstrap"
	aclpolicy "github.com/hashicorp/consul/command/acl/policy"
	aclpcreate "github.com/hashicorp/consul/command/acl/policy/create"
	aclpdelete "github.com/hashicorp/consul/command/acl/policy/delete"
	aclplist "github.com/hashicorp/consul/command/acl/policy/list"
	aclpread "github.com/hashicorp/consul/command/acl/policy/read"
	aclpupdate "github.com/hashicorp/consul/command/acl/policy/update"
	aclrole "github.com/hashicorp/consul/command/acl/role"
	aclrcreate "github.com/hashicorp/consul/command/acl/role/create"
	aclrdelete "github.com/hashicorp/consul/command/acl/role/delete"
	aclrlist "github.com/hashicorp/consul/command/acl/role/list"
	aclrread "github.com/hashicorp/consul/command/acl/role/read"
	aclrupdate "github.com/hashicorp/consul/command/acl/role/update"
	aclrules "github.com/hashicorp/consul/command/acl/rules"
	acltoken "github.com/hashicorp/consul/command/acl/token"
	acltclone "github.com/hashicorp/consul/command/acl/token/clone"
	acltcreate "github.com/hashicorp/consul/command/acl/token/create"
	acltdelete "github.com/hashicorp/consul/command/acl/token/delete"
	acltlist "github.com/hashicorp/consul/command/acl/token/list"
	acltread "github.com/hashicorp/consul/command/acl/token/read"
	acltupdate "github.com/hashicorp/consul/command/acl/token/update"
	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/command/catalog"
	catlistdc "github.com/hashicorp/consul/command/catalog/list/dc"
	catlistnodes "github.com/hashicorp/consul/command/catalog/list/nodes"
	catlistsvc "github.com/hashicorp/consul/command/catalog/list/services"
	"github.com/hashicorp/consul/command/config"
	configdelete "github.com/hashicorp/consul/command/config/delete"
	configlist "github.com/hashicorp/consul/command/config/list"
	configread "github.com/hashicorp/consul/command/config/read"
	configwrite "github.com/hashicorp/consul/command/config/write"
	"github.com/hashicorp/consul/command/connect"
	"github.com/hashicorp/consul/command/connect/ca"
	caget "github.com/hashicorp/consul/command/connect/ca/get"
	caset "github.com/hashicorp/consul/command/connect/ca/set"
	"github.com/hashicorp/consul/command/connect/envoy"
	pipebootstrap "github.com/hashicorp/consul/command/connect/envoy/pipe-bootstrap"
	"github.com/hashicorp/consul/command/connect/expose"
	"github.com/hashicorp/consul/command/connect/proxy"
	"github.com/hashicorp/consul/command/connect/redirecttraffic"
	"github.com/hashicorp/consul/command/debug"
	"github.com/hashicorp/consul/command/event"
	"github.com/hashicorp/consul/command/exec"
	"github.com/hashicorp/consul/command/forceleave"
	"github.com/hashicorp/consul/command/info"
	"github.com/hashicorp/consul/command/intention"
	ixncheck "github.com/hashicorp/consul/command/intention/check"
	ixncreate "github.com/hashicorp/consul/command/intention/create"
	ixndelete "github.com/hashicorp/consul/command/intention/delete"
	ixnget "github.com/hashicorp/consul/command/intention/get"
	ixnlist "github.com/hashicorp/consul/command/intention/list"
	ixnmatch "github.com/hashicorp/consul/command/intention/match"
	"github.com/hashicorp/consul/command/join"
	"github.com/hashicorp/consul/command/keygen"
	"github.com/hashicorp/consul/command/keyring"
	"github.com/hashicorp/consul/command/kv"
	kvdel "github.com/hashicorp/consul/command/kv/del"
	kvexp "github.com/hashicorp/consul/command/kv/exp"
	kvget "github.com/hashicorp/consul/command/kv/get"
	kvimp "github.com/hashicorp/consul/command/kv/imp"
	kvput "github.com/hashicorp/consul/command/kv/put"
	"github.com/hashicorp/consul/command/leave"
	"github.com/hashicorp/consul/command/lock"
	"github.com/hashicorp/consul/command/login"
	"github.com/hashicorp/consul/command/logout"
	"github.com/hashicorp/consul/command/maint"
	"github.com/hashicorp/consul/command/members"
	"github.com/hashicorp/consul/command/monitor"
	"github.com/hashicorp/consul/command/operator"
	operauto "github.com/hashicorp/consul/command/operator/autopilot"
	operautoget "github.com/hashicorp/consul/command/operator/autopilot/get"
	operautoset "github.com/hashicorp/consul/command/operator/autopilot/set"
	operautostate "github.com/hashicorp/consul/command/operator/autopilot/state"
	operraft "github.com/hashicorp/consul/command/operator/raft"
	operraftlist "github.com/hashicorp/consul/command/operator/raft/listpeers"
	operraftremove "github.com/hashicorp/consul/command/operator/raft/removepeer"
	"github.com/hashicorp/consul/command/reload"
	"github.com/hashicorp/consul/command/rtt"
	"github.com/hashicorp/consul/command/services"
	svcsderegister "github.com/hashicorp/consul/command/services/deregister"
	svcsregister "github.com/hashicorp/consul/command/services/register"
	"github.com/hashicorp/consul/command/snapshot"
	snapinspect "github.com/hashicorp/consul/command/snapshot/inspect"
	snaprestore "github.com/hashicorp/consul/command/snapshot/restore"
	snapsave "github.com/hashicorp/consul/command/snapshot/save"
	"github.com/hashicorp/consul/command/tls"
	tlsca "github.com/hashicorp/consul/command/tls/ca"
	tlscacreate "github.com/hashicorp/consul/command/tls/ca/create"
	tlscert "github.com/hashicorp/consul/command/tls/cert"
	tlscertcreate "github.com/hashicorp/consul/command/tls/cert/create"
	"github.com/hashicorp/consul/command/validate"
	"github.com/hashicorp/consul/command/version"
	"github.com/hashicorp/consul/command/watch"

	"os"
	"os/signal"
	"syscall"

	mcli "github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/cli"
)

// Factory is a function that returns a new instance of a CLI-sub command.
type Factory func(cli.Ui) (cli.Command, error)

// Entry is a function that returns a command's name and a Factory for that command.
type Entry func() (string, Factory)

// register will create an Entry from the passed in name and Factory.
func register(name string, fn Factory) Entry {
	return func() (string, Factory) {
		return name, fn
	}
}

func createCommands(ui cli.Ui, cmdEntries ...Entry) map[string]mcli.CommandFactory {
	m := make(map[string]mcli.CommandFactory)
	for _, fn := range cmdEntries {
		name, thisFn := fn()
		m[name] = func() (mcli.Command, error) {
			return thisFn(ui)
		}
	}
	return m
}

// CommandsFromRegistry returns a realized mapping of available CLI commands in a format that
// the CLI class can consume. This should be called after all registration is
// complete.
func CommandsFromRegistry(ui cli.Ui) map[string]mcli.CommandFactory {
	return createCommands(ui,
		register("acl", func(cli.Ui) (cli.Command, error) { return acl.New(), nil }),
		register("acl bootstrap", func(ui cli.Ui) (cli.Command, error) { return aclbootstrap.New(ui), nil }),
		register("acl policy", func(cli.Ui) (cli.Command, error) { return aclpolicy.New(), nil }),
		register("acl policy create", func(ui cli.Ui) (cli.Command, error) { return aclpcreate.New(ui), nil }),
		register("acl policy list", func(ui cli.Ui) (cli.Command, error) { return aclplist.New(ui), nil }),
		register("acl policy read", func(ui cli.Ui) (cli.Command, error) { return aclpread.New(ui), nil }),
		register("acl policy update", func(ui cli.Ui) (cli.Command, error) { return aclpupdate.New(ui), nil }),
		register("acl policy delete", func(ui cli.Ui) (cli.Command, error) { return aclpdelete.New(ui), nil }),
		register("acl translate-rules", func(ui cli.Ui) (cli.Command, error) { return aclrules.New(ui), nil }),
		register("acl set-agent-token", func(ui cli.Ui) (cli.Command, error) { return aclagent.New(ui), nil }),
		register("acl token", func(cli.Ui) (cli.Command, error) { return acltoken.New(), nil }),
		register("acl token create", func(ui cli.Ui) (cli.Command, error) { return acltcreate.New(ui), nil }),
		register("acl token clone", func(ui cli.Ui) (cli.Command, error) { return acltclone.New(ui), nil }),
		register("acl token list", func(ui cli.Ui) (cli.Command, error) { return acltlist.New(ui), nil }),
		register("acl token read", func(ui cli.Ui) (cli.Command, error) { return acltread.New(ui), nil }),
		register("acl token update", func(ui cli.Ui) (cli.Command, error) { return acltupdate.New(ui), nil }),
		register("acl token delete", func(ui cli.Ui) (cli.Command, error) { return acltdelete.New(ui), nil }),
		register("acl role", func(cli.Ui) (cli.Command, error) { return aclrole.New(), nil }),
		register("acl role create", func(ui cli.Ui) (cli.Command, error) { return aclrcreate.New(ui), nil }),
		register("acl role list", func(ui cli.Ui) (cli.Command, error) { return aclrlist.New(ui), nil }),
		register("acl role read", func(ui cli.Ui) (cli.Command, error) { return aclrread.New(ui), nil }),
		register("acl role update", func(ui cli.Ui) (cli.Command, error) { return aclrupdate.New(ui), nil }),
		register("acl role delete", func(ui cli.Ui) (cli.Command, error) { return aclrdelete.New(ui), nil }),
		register("acl auth-method", func(cli.Ui) (cli.Command, error) { return aclam.New(), nil }),
		register("acl auth-method create", func(ui cli.Ui) (cli.Command, error) { return aclamcreate.New(ui), nil }),
		register("acl auth-method list", func(ui cli.Ui) (cli.Command, error) { return aclamlist.New(ui), nil }),
		register("acl auth-method read", func(ui cli.Ui) (cli.Command, error) { return aclamread.New(ui), nil }),
		register("acl auth-method update", func(ui cli.Ui) (cli.Command, error) { return aclamupdate.New(ui), nil }),
		register("acl auth-method delete", func(ui cli.Ui) (cli.Command, error) { return aclamdelete.New(ui), nil }),
		register("acl binding-rule", func(cli.Ui) (cli.Command, error) { return aclbr.New(), nil }),
		register("acl binding-rule create", func(ui cli.Ui) (cli.Command, error) { return aclbrcreate.New(ui), nil }),
		register("acl binding-rule list", func(ui cli.Ui) (cli.Command, error) { return aclbrlist.New(ui), nil }),
		register("acl binding-rule read", func(ui cli.Ui) (cli.Command, error) { return aclbrread.New(ui), nil }),
		register("acl binding-rule update", func(ui cli.Ui) (cli.Command, error) { return aclbrupdate.New(ui), nil }),
		register("acl binding-rule delete", func(ui cli.Ui) (cli.Command, error) { return aclbrdelete.New(ui), nil }),
		register("agent", func(ui cli.Ui) (cli.Command, error) { return agent.New(ui), nil }),
		register("catalog", func(cli.Ui) (cli.Command, error) { return catalog.New(), nil }),
		register("catalog datacenters", func(ui cli.Ui) (cli.Command, error) { return catlistdc.New(ui), nil }),
		register("catalog nodes", func(ui cli.Ui) (cli.Command, error) { return catlistnodes.New(ui), nil }),
		register("catalog services", func(ui cli.Ui) (cli.Command, error) { return catlistsvc.New(ui), nil }),
		register("config", func(ui cli.Ui) (cli.Command, error) { return config.New(), nil }),
		register("config delete", func(ui cli.Ui) (cli.Command, error) { return configdelete.New(ui), nil }),
		register("config list", func(ui cli.Ui) (cli.Command, error) { return configlist.New(ui), nil }),
		register("config read", func(ui cli.Ui) (cli.Command, error) { return configread.New(ui), nil }),
		register("config write", func(ui cli.Ui) (cli.Command, error) { return configwrite.New(ui), nil }),
		register("connect", func(ui cli.Ui) (cli.Command, error) { return connect.New(), nil }),
		register("connect ca", func(ui cli.Ui) (cli.Command, error) { return ca.New(), nil }),
		register("connect ca get-config", func(ui cli.Ui) (cli.Command, error) { return caget.New(ui), nil }),
		register("connect ca set-config", func(ui cli.Ui) (cli.Command, error) { return caset.New(ui), nil }),
		register("connect proxy", func(ui cli.Ui) (cli.Command, error) { return proxy.New(ui, MakeShutdownCh()), nil }),
		register("connect envoy", func(ui cli.Ui) (cli.Command, error) { return envoy.New(ui), nil }),
		register("connect envoy pipe-bootstrap", func(ui cli.Ui) (cli.Command, error) { return pipebootstrap.New(ui), nil }),
		register("connect expose", func(ui cli.Ui) (cli.Command, error) { return expose.New(ui), nil }),
		register("connect redirect-traffic", func(ui cli.Ui) (cli.Command, error) { return redirecttraffic.New(ui), nil }),
		register("debug", func(ui cli.Ui) (cli.Command, error) { return debug.New(ui), nil }),
		register("event", func(ui cli.Ui) (cli.Command, error) { return event.New(ui), nil }),
		register("exec", func(ui cli.Ui) (cli.Command, error) { return exec.New(ui, MakeShutdownCh()), nil }),
		register("force-leave", func(ui cli.Ui) (cli.Command, error) { return forceleave.New(ui), nil }),
		register("info", func(ui cli.Ui) (cli.Command, error) { return info.New(ui), nil }),
		register("intention", func(ui cli.Ui) (cli.Command, error) { return intention.New(), nil }),
		register("intention check", func(ui cli.Ui) (cli.Command, error) { return ixncheck.New(ui), nil }),
		register("intention create", func(ui cli.Ui) (cli.Command, error) { return ixncreate.New(ui), nil }),
		register("intention delete", func(ui cli.Ui) (cli.Command, error) { return ixndelete.New(ui), nil }),
		register("intention get", func(ui cli.Ui) (cli.Command, error) { return ixnget.New(ui), nil }),
		register("intention list", func(ui cli.Ui) (cli.Command, error) { return ixnlist.New(ui), nil }),
		register("intention match", func(ui cli.Ui) (cli.Command, error) { return ixnmatch.New(ui), nil }),
		register("join", func(ui cli.Ui) (cli.Command, error) { return join.New(ui), nil }),
		register("keygen", func(ui cli.Ui) (cli.Command, error) { return keygen.New(ui), nil }),
		register("keyring", func(ui cli.Ui) (cli.Command, error) { return keyring.New(ui), nil }),
		register("kv", func(cli.Ui) (cli.Command, error) { return kv.New(), nil }),
		register("kv delete", func(ui cli.Ui) (cli.Command, error) { return kvdel.New(ui), nil }),
		register("kv export", func(ui cli.Ui) (cli.Command, error) { return kvexp.New(ui), nil }),
		register("kv get", func(ui cli.Ui) (cli.Command, error) { return kvget.New(ui), nil }),
		register("kv import", func(ui cli.Ui) (cli.Command, error) { return kvimp.New(ui), nil }),
		register("kv put", func(ui cli.Ui) (cli.Command, error) { return kvput.New(ui), nil }),
		register("leave", func(ui cli.Ui) (cli.Command, error) { return leave.New(ui), nil }),
		register("lock", func(ui cli.Ui) (cli.Command, error) { return lock.New(ui, MakeShutdownCh()), nil }),
		register("login", func(ui cli.Ui) (cli.Command, error) { return login.New(ui), nil }),
		register("logout", func(ui cli.Ui) (cli.Command, error) { return logout.New(ui), nil }),
		register("maint", func(ui cli.Ui) (cli.Command, error) { return maint.New(ui), nil }),
		register("members", func(ui cli.Ui) (cli.Command, error) { return members.New(ui), nil }),
		register("monitor", func(ui cli.Ui) (cli.Command, error) { return monitor.New(ui, MakeShutdownCh()), nil }),
		register("operator", func(cli.Ui) (cli.Command, error) { return operator.New(), nil }),
		register("operator autopilot", func(cli.Ui) (cli.Command, error) { return operauto.New(), nil }),
		register("operator autopilot get-config", func(ui cli.Ui) (cli.Command, error) { return operautoget.New(ui), nil }),
		register("operator autopilot set-config", func(ui cli.Ui) (cli.Command, error) { return operautoset.New(ui), nil }),
		register("operator autopilot state", func(ui cli.Ui) (cli.Command, error) { return operautostate.New(ui), nil }),
		register("operator raft", func(cli.Ui) (cli.Command, error) { return operraft.New(), nil }),
		register("operator raft list-peers", func(ui cli.Ui) (cli.Command, error) { return operraftlist.New(ui), nil }),
		register("operator raft remove-peer", func(ui cli.Ui) (cli.Command, error) { return operraftremove.New(ui), nil }),
		register("reload", func(ui cli.Ui) (cli.Command, error) { return reload.New(ui), nil }),
		register("rtt", func(ui cli.Ui) (cli.Command, error) { return rtt.New(ui), nil }),
		register("services", func(cli.Ui) (cli.Command, error) { return services.New(), nil }),
		register("services register", func(ui cli.Ui) (cli.Command, error) { return svcsregister.New(ui), nil }),
		register("services deregister", func(ui cli.Ui) (cli.Command, error) { return svcsderegister.New(ui), nil }),
		register("snapshot", func(cli.Ui) (cli.Command, error) { return snapshot.New(), nil }),
		register("snapshot inspect", func(ui cli.Ui) (cli.Command, error) { return snapinspect.New(ui), nil }),
		register("snapshot restore", func(ui cli.Ui) (cli.Command, error) { return snaprestore.New(ui), nil }),
		register("snapshot save", func(ui cli.Ui) (cli.Command, error) { return snapsave.New(ui), nil }),
		register("tls", func(ui cli.Ui) (cli.Command, error) { return tls.New(), nil }),
		register("tls ca", func(ui cli.Ui) (cli.Command, error) { return tlsca.New(), nil }),
		register("tls ca create", func(ui cli.Ui) (cli.Command, error) { return tlscacreate.New(ui), nil }),
		register("tls cert", func(ui cli.Ui) (cli.Command, error) { return tlscert.New(), nil }),
		register("tls cert create", func(ui cli.Ui) (cli.Command, error) { return tlscertcreate.New(ui), nil }),
		register("validate", func(ui cli.Ui) (cli.Command, error) { return validate.New(ui), nil }),
		register("version", func(ui cli.Ui) (cli.Command, error) { return version.New(ui), nil }),
		register("watch", func(ui cli.Ui) (cli.Command, error) { return watch.New(ui, MakeShutdownCh()), nil }),
	)
}

// MakeShutdownCh returns a channel that can be used for shutdown notifications
// for commands. This channel will send a message for every interrupt or SIGTERM
// received.
// Deprecated: use signal.NotifyContext
func MakeShutdownCh() <-chan struct{} {
	resultCh := make(chan struct{})
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		for {
			<-signalCh
			resultCh <- struct{}{}
		}
	}()

	return resultCh
}
