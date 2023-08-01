# Adding a Consul Config Field

This is a checklist of all the places you need to update when adding a new field
to config. There may be a few other special cases not included but this covers
the majority of configs.

We suggest you copy the raw markdown into a gist or local file and check them
off as you go (you can mark them as done by replace `[ ]` with `[x]` so github
renders them as checked). Then **please include the completed lists you worked
through in your PR description**.

Examples of special cases this doesn't cover are:
 - If the config needs special treatment like a different default in `-dev` mode
   or differences between OSS and Enterprise.
 - If custom logic is needed to support backwards compatibility when changing
   syntax or semantics of anything

There are four specific cases covered with increasing complexity:
 1. adding a simple config field only used by client agents
 1. adding a CLI flag to mirror that config field
 1. adding a config field that needs to be used in Consul servers
 1. adding a field to the Service Definition

## Adding a Simple Config Field for Client Agents

 - [ ] Add the field to the Config struct (or an appropriate sub-struct) in
   `agent/config/config.go`.
 - [ ] Add the field to the actual RuntimeConfig struct in
   `agent/config/runtime.go`.
 - [ ] Add an appropriate parser/setter in `agent/config/builder.go` to
   translate.
 - [ ] Add the new field with a random value to both the JSON and HCL files in
   `agent/config/testdata/full-config.*`, which should cause the test to fail.
   Then update the expected value in `TestLoad_FullConfig` in
   `agent/config/runtime_test.go` to make the test pass again.
 - [ ] Run `go test -run TestRuntimeConfig_Sanitize ./agent/config -update` to update
   the expected value for `TestRuntimeConfig_Sanitize`. Look at `git diff` to
   make sure the value changed as you expect.
 - [ ] **If** your new config field needed some validation as it's only valid in
   some cases or with some values (often true).
      - [ ] Add validation to Validate in `agent/config/builder.go`.
      - [ ] Add a test case to the table test `TestLoad_IntegrationWithFlags` in
        `agent/config/runtime_test.go`.
 - [ ] **If** your new config field needs a non-zero-value default.
      - [ ] Add that to `DefaultSource` in `agent/config/defaults.go`.
      - [ ] Add a test case to the table test `TestLoad_IntegrationWithFlags` in
        `agent/config/runtime_test.go`.
      - [ ] If the config needs to be defaulted for the test server used in unit tests,
            also add it to `DefaultConfig()` in `agent/consul/config.go`.
 - [ ] **If** your config should take effect on a reload/HUP.
      - [ ] Add necessary code to to trigger a safe (locked or atomic) update to
        any state the feature needs changing. This needs to be added to one or
        more of the following places:
         - `ReloadConfig` in `agent/agent.go` if it needs to affect the local
           client state or another client agent component.
         - `ReloadConfig` in `agent/consul/client.go` if it needs to affect
           state for client agent's RPC client.
      - [ ] Add a test to `agent/agent_test.go` similar to others with prefix
        `TestAgent_reloadConfig*`.
 - [ ] Add documentation to `website/content/docs/agent/config/config-files.mdx`.

Done! You can now use your new field in a client agent by accessing
`s.agent.Config.<FieldName>`.

If you need a CLI flag, access to the variable in a Server context, or touched
the Service Definition, make sure you continue on to follow the appropriate
checklists below.

## Adding a CLI Flag Corresponding to the new Field
If the config field also needs a CLI flag, then follow these steps.

 - [ ] Do all of the steps in [Adding a Simple Config
   Field For Client Agents](#adding-a-simple-config-field-for-client-agents).
 - [ ] Add the new flag to `agent/config/flags.go`.
 - [ ] Add a test case to TestParseFlags in `agent/config/flag_test.go`.
 - [ ] Add a test case (or extend one if appropriate) to the table test
   `TestLoad_IntegrationWithFlags` in `agent/config/runtime_test.go` to ensure setting the
   flag works.
 - [ ] Add flag (as well as config file) documentation to
   `website/source/docs/agent/config/config-files.mdx` and `website/source/docs/agent/config/cli-flags.mdx`.

## Adding a Simple Config Field for Servers
Consul servers have a separate Config struct for reasons. Note that Consul
server agents are actually also client agents, so in some cases config that is
only destined for servers doesn't need to follow this checklist provided it's
only needed during the bootstrapping of the server (which is done in code shared
by both server and client components in `agent.go`). For example WAN Gossip
configs are only valid on server agents but since WAN Gossip is setup in
`agent.go` they don't need to follow this checklist. The simplest (and mostly
accurate) rule is:

> If you need to access the config field from code in  `agent/consul` (e.g. RPC
> endpoints), then you need to follow this. If it's only in `agent` (e.g. HTTP
> endpoints or agent startup) you don't.

A final word of warning - **you should never need to pass config into the FSM
(`agent/consul/fsm`) or state store (`agent/consul/state`)**. Doing so is **_very
dangerous_** and can violate consistency guarantees and corrupt databases. If
you think you need this then please discuss the design with the Consul team
before writing code!

Consul's server components for historical reasons don't use the `RuntimeConfig`
struct they have their own struct called `Config` in `agent/consul/config.go`.

 - [ ] Do all of the steps in [Adding a Simple Config
   Field For Client Agents](#adding-a-simple-config-field-for-client-agents).
 - [ ] Add the new field to Config struct in `agent/consul/config.go`
 - [ ] Add code to set the values from the `RuntimeConfig` in `newConsulConfig` method in `agent/agent.go`
 - [ ] **If needed**, add a test to `agent_test.go` if there is some non-trivial
   behavior in the code you added in the previous step. We tend not to test
   simple assignments from one to the other since these are typically caught by
   higher-level tests of the actual functionality that matters but some examples
   can be found prefixed with `TestAgent_consulConfig*`
 - [ ] **If** your config should take effect on a reload/HUP
      - [ ] Add necessary code to `ReloadConfig` in `agent/consul/server.go` this
        needs to be adequately synchronized with any readers of the state being
        updated.
       - [ ] Add a new test or a new assertion to `TestServer_ReloadConfig`

You can now access that field from `s.srv.config.<FieldName>` inside an RPC
handler.

## Adding a New Field to Service Definition
The [Service Definition](https://developer.hashicorp.com/consul/docs/services/services) syntax
appears both in Consul config files but also in the `/v1/agent/service/register`
API.

For wonderful historical reasons, our config files have always used `snake_case`
attribute names in both JSON and HCL (even before we supported HCL!!) while our
API uses `CamelCase`.

Because we want documentation examples to work in both config files and API
bodies to avoid needless confusion, we have to accept both snake case and camel
case field names for the service definition.

Finally, adding a field to the service definition implies adding the field to
several internal structs and to all API outputs that display services from the
catalog. That explains the multiple layers needed below.

This list assumes a new field in the base service definition struct. Adding new
fields to health checks is similar but mostly needs `HealthCheck` structs and
methods updating instead. Adding fields to embedded structs like `ProxyConfig`
is largely the same pattern but may need different test methods etc. updating.

 - [ ] Do all of the steps in [Adding a Simple Config
   Field For Client Agents](#adding-a-simple-config-field-for-client-agents).
 - [ ] `agent/structs` package
      - [ ] Add the field to `ServiceDefinition` (`service_definition.go`)
      - [ ] Add the field to `NodeService` (`structs.go`)
      - [ ] Add the field to `ServiceNode` (`structs.go`)
      - [ ] Update `ServiceDefinition.ToNodeService` to translate the field
      - [ ] Update `NodeService.ToServiceNode` to translate the field
      - [ ] Update `ServiceNode.ToNodeService` to translate the field
      - [ ] Update `TestStructs_ServiceNode_Conversions`
      - [ ] Update `ServiceNode.PartialClone`
      - [ ] Update `TestStructs_ServiceNode_PartialClone` (`structs_test.go`)
      - [ ] If needed, update `NodeService.Validate` to ensure the field value is
        reasonable
      - [ ] Add test like `TestStructs_NodeService_Validate*` in
        `structs_test.go`
      - [ ] Add comparison in `NodeService.IsSame`
      - [ ] Update `TestStructs_NodeService_IsSame`
      - [ ] Add comparison in `ServiceNode.IsSameService`
      - [ ] Update `TestStructs_ServiceNode_IsSameService`
      - [ ] **If** your field name has MultipleWords,
          - [ ] Add it to the `aux` inline struct in
            `ServiceDefinition.UnmarshalJSON` (`service_defintion.go`). 
            - Note: if the field is embedded higher up in a nested struct,
              follow the chain and update the necessary struct's `UnmarshalJSON`
              method - you may need to add one if there are no other case
              transformations being done, copy and existing example. 
            - Note: the tests that exercise this are in agent endpoint for
              historical reasons (this is where the translation used to happen).
 - [ ] `agent` package
      - [ ] Update `testAgent_RegisterService` and/or add a new test to ensure
        your fields register correctly via API (`agent_endpoint_test.go`)
      - [ ] **If** your field name has MultipleWords,
          - [ ] Update `testAgent_RegisterService_TranslateKeys` to include
            examples with it set in `snake_case` and ensure it is parsed
            correctly. Run this via `TestAgent_RegisterService_TranslateKeys`
            (agent_endpoint_test.go).
 - [ ] `api` package
      - [ ] Add the field to `AgentService` (`agent.go`)
      - [ ] Add/update an appropriate test in `agent_test.go`
        - (Note you need to use `make test` or ensure the `consul` binary on
          your `$PATH` is a build with your new field - usually `make dev`
          ensures this unless you're path is funky or you have a consul binary
          even further up the shell's `$PATH`).
 - [ ] Docs
      - [ ] Update docs in `website/source/docs/agent/services.html.md`
      - [ ] Consider if it's worth adding examples to feature docs or API docs
        that show the new field's usage.

Note that although the new field will show up in the API output of
`/agent/services` , `/catalog/services` and `/health/services`, those tests
right now don't exercise anything that's super useful unless custom logic is
required since they don't even encode the response object as JSON and just
assert on the structs you already modified. If custom presentation logic is
needed, tests for these endpoints might be warranted too. It's usual to use
`omit-empty` for new fields that will typically not be used by existing
registrations although we don't currently test for that systematically.
