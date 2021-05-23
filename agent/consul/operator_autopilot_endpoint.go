package consul

import (
	"fmt"

	autopilot "github.com/hashicorp/raft-autopilot"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// AutopilotGetConfiguration is used to retrieve the current Autopilot configuration.
func (op *Operator) AutopilotGetConfiguration(args *structs.DCSpecificRequest, reply *structs.AutopilotConfig) error {
	if done, err := op.srv.ForwardRPC("Operator.AutopilotGetConfiguration", args, reply); done {
		return err
	}

	// This action requires operator read access.
	identity, rule, err := op.srv.ResolveTokenToIdentityAndAuthorizer(args.Token)
	if err != nil {
		return err
	}
	if err := op.srv.validateEnterpriseToken(identity); err != nil {
		return err
	}
	if rule != nil && rule.OperatorRead(nil) != acl.Allow {
		return acl.PermissionDenied("Missing operator:read permissions")
	}

	state := op.srv.fsm.State()
	_, config, err := state.AutopilotConfig()
	if err != nil {
		return err
	}
	if config == nil {
		return fmt.Errorf("autopilot config not initialized yet")
	}

	*reply = *config

	return nil
}

// AutopilotSetConfiguration is used to set the current Autopilot configuration.
func (op *Operator) AutopilotSetConfiguration(args *structs.AutopilotSetConfigRequest, reply *bool) error {
	if done, err := op.srv.ForwardRPC("Operator.AutopilotSetConfiguration", args, reply); done {
		return err
	}

	// This action requires operator write access.
	identity, rule, err := op.srv.ResolveTokenToIdentityAndAuthorizer(args.Token)
	if err != nil {
		return err
	}
	if err := op.srv.validateEnterpriseToken(identity); err != nil {
		return err
	}
	if rule != nil && rule.OperatorWrite(nil) != acl.Allow {
		return acl.PermissionDenied("Missing operator:write permissions")
	}

	// Apply the update
	resp, err := op.srv.raftApply(structs.AutopilotRequestType, args)
	if err != nil {
		return fmt.Errorf("raft apply failed: %w", err)
	}

	// Check if the return type is a bool.
	if respBool, ok := resp.(bool); ok {
		*reply = respBool
	}
	return nil
}

// ServerHealth is used to get the current health of the servers.
func (op *Operator) ServerHealth(args *structs.DCSpecificRequest, reply *structs.AutopilotHealthReply) error {
	// This must be sent to the leader, so we fix the args since we are
	// re-using a structure where we don't support all the options.
	args.RequireConsistent = true
	args.AllowStale = false
	if done, err := op.srv.ForwardRPC("Operator.ServerHealth", args, reply); done {
		return err
	}

	// This action requires operator read access.
	identity, rule, err := op.srv.ResolveTokenToIdentityAndAuthorizer(args.Token)
	if err != nil {
		return err
	}
	if err := op.srv.validateEnterpriseToken(identity); err != nil {
		return err
	}
	if rule != nil && rule.OperatorRead(nil) != acl.Allow {
		return acl.PermissionDenied("Missing operator:read permissions")
	}

	state := op.srv.autopilot.GetState()

	if state == nil {
		// this behavior seems odd but its functionally equivalent to 1.8.5 where if
		// autopilot didn't have a health reply yet it would just return no error
		return nil
	}

	health := structs.AutopilotHealthReply{
		Healthy:          state.Healthy,
		FailureTolerance: state.FailureTolerance,
	}

	for _, srv := range state.Servers {
		srvHealth := structs.AutopilotServerHealth{
			ID:          string(srv.Server.ID),
			Name:        srv.Server.Name,
			Address:     string(srv.Server.Address),
			Version:     srv.Server.Version,
			Leader:      srv.State == autopilot.RaftLeader,
			Voter:       srv.State == autopilot.RaftLeader || srv.State == autopilot.RaftVoter,
			LastContact: srv.Stats.LastContact,
			LastTerm:    srv.Stats.LastTerm,
			LastIndex:   srv.Stats.LastIndex,
			Healthy:     srv.Health.Healthy,
			StableSince: srv.Health.StableSince,
		}

		switch srv.Server.NodeStatus {
		case autopilot.NodeAlive:
			srvHealth.SerfStatus = serf.StatusAlive
		case autopilot.NodeLeft:
			srvHealth.SerfStatus = serf.StatusLeft
		case autopilot.NodeFailed:
			srvHealth.SerfStatus = serf.StatusFailed
		default:
			srvHealth.SerfStatus = serf.StatusNone
		}

		health.Servers = append(health.Servers, srvHealth)
	}

	*reply = health
	return nil
}

func (op *Operator) AutopilotState(args *structs.DCSpecificRequest, reply *autopilot.State) error {
	// This must be sent to the leader, so we fix the args since we are
	// re-using a structure where we don't support all the options.
	args.RequireConsistent = true
	args.AllowStale = false
	if done, err := op.srv.ForwardRPC("Operator.AutopilotState", args, reply); done {
		return err
	}

	// This action requires operator read access.
	identity, rule, err := op.srv.ResolveTokenToIdentityAndAuthorizer(args.Token)
	if err != nil {
		return err
	}
	if err := op.srv.validateEnterpriseToken(identity); err != nil {
		return err
	}
	if rule != nil && rule.OperatorRead(nil) != acl.Allow {
		return acl.PermissionDenied("Missing operator:read permissions")
	}

	state := op.srv.autopilot.GetState()
	if state == nil {
		return fmt.Errorf("Failed to get autopilot state: no state found")
	}

	*reply = *state
	return nil
}
