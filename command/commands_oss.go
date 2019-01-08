package command

import (
	"github.com/hashicorp/consul/command/acl"
	aclagent "github.com/hashicorp/consul/command/acl/agenttokens"
	aclbootstrap "github.com/hashicorp/consul/command/acl/bootstrap"
	aclpolicy "github.com/hashicorp/consul/command/acl/policy"
	aclpcreate "github.com/hashicorp/consul/command/acl/policy/create"
	aclpdelete "github.com/hashicorp/consul/command/acl/policy/delete"
	aclplist "github.com/hashicorp/consul/command/acl/policy/list"
	aclpread "github.com/hashicorp/consul/command/acl/policy/read"
	aclpupdate "github.com/hashicorp/consul/command/acl/policy/update"
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
	"github.com/hashicorp/consul/command/connect"
	"github.com/hashicorp/consul/command/connect/ca"
	caget "github.com/hashicorp/consul/command/connect/ca/get"
	caset "github.com/hashicorp/consul/command/connect/ca/set"
	"github.com/hashicorp/consul/command/connect/envoy"
	"github.com/hashicorp/consul/command/connect/proxy"
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
	"github.com/hashicorp/consul/command/maint"
	"github.com/hashicorp/consul/command/members"
	"github.com/hashicorp/consul/command/monitor"
	"github.com/hashicorp/consul/command/operator"
	operauto "github.com/hashicorp/consul/command/operator/autopilot"
	operautoget "github.com/hashicorp/consul/command/operator/autopilot/get"
	operautoset "github.com/hashicorp/consul/command/operator/autopilot/set"
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
	consulversion "github.com/hashicorp/consul/version"

	"github.com/mitchellh/cli"
)

func init() {
	rev := consulversion.GitCommit
	ver := consulversion.Version
	verPre := consulversion.VersionPrerelease
	verHuman := consulversion.GetHumanVersion()

	Register("acl", func(cli.Ui) (cli.Command, error) { return acl.New(), nil })
	Register("acl bootstrap", func(ui cli.Ui) (cli.Command, error) { return aclbootstrap.New(ui), nil })
	Register("acl policy", func(cli.Ui) (cli.Command, error) { return aclpolicy.New(), nil })
	Register("acl policy create", func(ui cli.Ui) (cli.Command, error) { return aclpcreate.New(ui), nil })
	Register("acl policy list", func(ui cli.Ui) (cli.Command, error) { return aclplist.New(ui), nil })
	Register("acl policy read", func(ui cli.Ui) (cli.Command, error) { return aclpread.New(ui), nil })
	Register("acl policy update", func(ui cli.Ui) (cli.Command, error) { return aclpupdate.New(ui), nil })
	Register("acl policy delete", func(ui cli.Ui) (cli.Command, error) { return aclpdelete.New(ui), nil })
	Register("acl translate-rules", func(ui cli.Ui) (cli.Command, error) { return aclrules.New(ui), nil })
	Register("acl set-agent-token", func(ui cli.Ui) (cli.Command, error) { return aclagent.New(ui), nil })
	Register("acl token", func(cli.Ui) (cli.Command, error) { return acltoken.New(), nil })
	Register("acl token create", func(ui cli.Ui) (cli.Command, error) { return acltcreate.New(ui), nil })
	Register("acl token clone", func(ui cli.Ui) (cli.Command, error) { return acltclone.New(ui), nil })
	Register("acl token list", func(ui cli.Ui) (cli.Command, error) { return acltlist.New(ui), nil })
	Register("acl token read", func(ui cli.Ui) (cli.Command, error) { return acltread.New(ui), nil })
	Register("acl token update", func(ui cli.Ui) (cli.Command, error) { return acltupdate.New(ui), nil })
	Register("acl token delete", func(ui cli.Ui) (cli.Command, error) { return acltdelete.New(ui), nil })
	Register("agent", func(ui cli.Ui) (cli.Command, error) {
		return agent.New(ui, rev, ver, verPre, verHuman, make(chan struct{})), nil
	})
	Register("catalog", func(cli.Ui) (cli.Command, error) { return catalog.New(), nil })
	Register("catalog datacenters", func(ui cli.Ui) (cli.Command, error) { return catlistdc.New(ui), nil })
	Register("catalog nodes", func(ui cli.Ui) (cli.Command, error) { return catlistnodes.New(ui), nil })
	Register("catalog services", func(ui cli.Ui) (cli.Command, error) { return catlistsvc.New(ui), nil })
	Register("connect", func(ui cli.Ui) (cli.Command, error) { return connect.New(), nil })
	Register("connect ca", func(ui cli.Ui) (cli.Command, error) { return ca.New(), nil })
	Register("connect ca get-config", func(ui cli.Ui) (cli.Command, error) { return caget.New(ui), nil })
	Register("connect ca set-config", func(ui cli.Ui) (cli.Command, error) { return caset.New(ui), nil })
	Register("connect proxy", func(ui cli.Ui) (cli.Command, error) { return proxy.New(ui, MakeShutdownCh()), nil })
	Register("connect envoy", func(ui cli.Ui) (cli.Command, error) { return envoy.New(ui), nil })
	Register("debug", func(ui cli.Ui) (cli.Command, error) { return debug.New(ui, MakeShutdownCh()), nil })
	Register("event", func(ui cli.Ui) (cli.Command, error) { return event.New(ui), nil })
	Register("exec", func(ui cli.Ui) (cli.Command, error) { return exec.New(ui, MakeShutdownCh()), nil })
	Register("force-leave", func(ui cli.Ui) (cli.Command, error) { return forceleave.New(ui), nil })
	Register("info", func(ui cli.Ui) (cli.Command, error) { return info.New(ui), nil })
	Register("intention", func(ui cli.Ui) (cli.Command, error) { return intention.New(), nil })
	Register("intention check", func(ui cli.Ui) (cli.Command, error) { return ixncheck.New(ui), nil })
	Register("intention create", func(ui cli.Ui) (cli.Command, error) { return ixncreate.New(ui), nil })
	Register("intention delete", func(ui cli.Ui) (cli.Command, error) { return ixndelete.New(ui), nil })
	Register("intention get", func(ui cli.Ui) (cli.Command, error) { return ixnget.New(ui), nil })
	Register("intention match", func(ui cli.Ui) (cli.Command, error) { return ixnmatch.New(ui), nil })
	Register("join", func(ui cli.Ui) (cli.Command, error) { return join.New(ui), nil })
	Register("keygen", func(ui cli.Ui) (cli.Command, error) { return keygen.New(ui), nil })
	Register("keyring", func(ui cli.Ui) (cli.Command, error) { return keyring.New(ui), nil })
	Register("kv", func(cli.Ui) (cli.Command, error) { return kv.New(), nil })
	Register("kv delete", func(ui cli.Ui) (cli.Command, error) { return kvdel.New(ui), nil })
	Register("kv export", func(ui cli.Ui) (cli.Command, error) { return kvexp.New(ui), nil })
	Register("kv get", func(ui cli.Ui) (cli.Command, error) { return kvget.New(ui), nil })
	Register("kv import", func(ui cli.Ui) (cli.Command, error) { return kvimp.New(ui), nil })
	Register("kv put", func(ui cli.Ui) (cli.Command, error) { return kvput.New(ui), nil })
	Register("leave", func(ui cli.Ui) (cli.Command, error) { return leave.New(ui), nil })
	Register("lock", func(ui cli.Ui) (cli.Command, error) { return lock.New(ui), nil })
	Register("maint", func(ui cli.Ui) (cli.Command, error) { return maint.New(ui), nil })
	Register("members", func(ui cli.Ui) (cli.Command, error) { return members.New(ui), nil })
	Register("monitor", func(ui cli.Ui) (cli.Command, error) { return monitor.New(ui, MakeShutdownCh()), nil })
	Register("operator", func(cli.Ui) (cli.Command, error) { return operator.New(), nil })
	Register("operator autopilot", func(cli.Ui) (cli.Command, error) { return operauto.New(), nil })
	Register("operator autopilot get-config", func(ui cli.Ui) (cli.Command, error) { return operautoget.New(ui), nil })
	Register("operator autopilot set-config", func(ui cli.Ui) (cli.Command, error) { return operautoset.New(ui), nil })
	Register("operator raft", func(cli.Ui) (cli.Command, error) { return operraft.New(), nil })
	Register("operator raft list-peers", func(ui cli.Ui) (cli.Command, error) { return operraftlist.New(ui), nil })
	Register("operator raft remove-peer", func(ui cli.Ui) (cli.Command, error) { return operraftremove.New(ui), nil })
	Register("reload", func(ui cli.Ui) (cli.Command, error) { return reload.New(ui), nil })
	Register("rtt", func(ui cli.Ui) (cli.Command, error) { return rtt.New(ui), nil })
	Register("services", func(cli.Ui) (cli.Command, error) { return services.New(), nil })
	Register("services register", func(ui cli.Ui) (cli.Command, error) { return svcsregister.New(ui), nil })
	Register("services deregister", func(ui cli.Ui) (cli.Command, error) { return svcsderegister.New(ui), nil })
	Register("snapshot", func(cli.Ui) (cli.Command, error) { return snapshot.New(), nil })
	Register("snapshot inspect", func(ui cli.Ui) (cli.Command, error) { return snapinspect.New(ui), nil })
	Register("snapshot restore", func(ui cli.Ui) (cli.Command, error) { return snaprestore.New(ui), nil })
	Register("snapshot save", func(ui cli.Ui) (cli.Command, error) { return snapsave.New(ui), nil })
	Register("tls", func(ui cli.Ui) (cli.Command, error) { return tls.New(), nil })
	Register("tls ca", func(ui cli.Ui) (cli.Command, error) { return tlsca.New(), nil })
	Register("tls ca create", func(ui cli.Ui) (cli.Command, error) { return tlscacreate.New(ui), nil })
	Register("tls cert", func(ui cli.Ui) (cli.Command, error) { return tlscert.New(), nil })
	Register("tls cert create", func(ui cli.Ui) (cli.Command, error) { return tlscertcreate.New(ui), nil })
	Register("validate", func(ui cli.Ui) (cli.Command, error) { return validate.New(ui), nil })
	Register("version", func(ui cli.Ui) (cli.Command, error) { return version.New(ui, verHuman), nil })
	Register("watch", func(ui cli.Ui) (cli.Command, error) { return watch.New(ui, MakeShutdownCh()), nil })
}
