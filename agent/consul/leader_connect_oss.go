// +build !consulent

package consul

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
)

const intentionUpgradeCleanupRoutineName = "intention cleanup"

func (s *Server) startConnectLeaderEnterprise() {
	if s.config.PrimaryDatacenter != s.config.Datacenter {
		// intention cleanup should only run in the primary
		return
	}
	s.leaderRoutineManager.Start(intentionUpgradeCleanupRoutineName, s.runIntentionUpgradeCleanup)
}

func (s *Server) stopConnectLeaderEnterprise() {
	// will be a no-op when not started
	s.leaderRoutineManager.Stop(intentionUpgradeCleanupRoutineName)
}

func (s *Server) runIntentionUpgradeCleanup(ctx context.Context) error {
	// TODO(rb): handle retry?

	// Remove any intention in OSS that happened to have used a non-default
	// namespace.
	//
	// The one exception is that if we find wildcards namespaces we "upgrade"
	// them to "default" if there isn't already an existing intention.
	_, ixns, err := s.fsm.State().Intentions(nil, structs.WildcardEnterpriseMeta())
	if err != nil {
		return fmt.Errorf("failed to list intentions: %v", err)
	}

	// default/<foo> => default/<foo> || OK
	// default/*     => default/<foo> || OK
	// */*           => default/<foo> || becomes: default/*     => default/<foo>
	// default/<foo> => default/*     || OK
	// default/*     => default/*     || OK
	// */*           => default/*     || becomes: default/*     => default/*
	// default/<foo> => */*           || becomes: default/<foo> => default/*
	// default/*     => */*           || becomes: default/*     => default/*
	// */*           => */*           || becomes: default/*     => default/*

	type intentionName struct {
		SourceNS, SourceName           string
		DestinationNS, DestinationName string
	}

	var (
		retained    = make(map[intentionName]struct{})
		tryUpgrades = make(map[intentionName]*structs.Intention)
		removeIDs   []string
	)
	for _, ixn := range ixns {
		srcNS := strings.ToLower(ixn.SourceNS)
		if srcNS == "" {
			srcNS = structs.IntentionDefaultNamespace
		}
		dstNS := strings.ToLower(ixn.DestinationNS)
		if dstNS == "" {
			dstNS = structs.IntentionDefaultNamespace
		}

		if srcNS == structs.IntentionDefaultNamespace && dstNS == structs.IntentionDefaultNamespace {
			name := intentionName{
				srcNS, ixn.SourceName,
				dstNS, ixn.DestinationName,
			}
			retained[name] = struct{}{}
			continue // a-ok for OSS
		}

		// If anything is wildcarded, attempt to reify it as "default".
		if srcNS == structs.WildcardSpecifier || dstNS == structs.WildcardSpecifier {
			updated := ixn.Clone()
			if srcNS == structs.WildcardSpecifier {
				updated.SourceNS = structs.IntentionDefaultNamespace
			}
			if dstNS == structs.WildcardSpecifier {
				updated.DestinationNS = structs.IntentionDefaultNamespace
			}

			// Run parts of the checks in Intention.prepareApplyUpdate.

			// We always update the updatedat field.
			updated.UpdatedAt = time.Now().UTC()

			// Set the precedence
			updated.UpdatePrecedence()

			// make sure we set the hash prior to raft application
			updated.SetHash()

			name := intentionName{
				updated.SourceNS, updated.SourceName,
				updated.DestinationNS, updated.DestinationName,
			}
			tryUpgrades[name] = updated
		} else {
			removeIDs = append(removeIDs, ixn.ID)
		}
	}

	for name, updated := range tryUpgrades {
		if _, collision := retained[name]; collision {
			// The update we wanted to do would collide with an existing intention
			// so delete our original wildcard intention instead.
			removeIDs = append(removeIDs, updated.ID)
		} else {
			req := structs.IntentionRequest{
				Op:        structs.IntentionOpUpdate,
				Intention: updated,
			}
			if _, err := s.raftApply(structs.IntentionRequestType, &req); err != nil {
				return fmt.Errorf("failed to remove wildcard namespaces from intention %q: %v", updated.ID, err)
			}
		}
	}

	for _, id := range removeIDs {
		req := structs.IntentionRequest{
			Op: structs.IntentionOpDelete,
			Intention: &structs.Intention{
				ID: id,
			},
		}
		if _, err := s.raftApply(structs.IntentionRequestType, &req); err != nil {
			return fmt.Errorf("failed to remove intention with invalid namespace %q: %v", id, err)
		}
	}

	return nil // transition complete
}
