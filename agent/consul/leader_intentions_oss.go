// +build !consulent

package consul

import (
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

func migrateIntentionsToConfigEntries(ixns structs.Intentions) []*structs.ServiceIntentionsConfigEntry {
	// Remove any intention in OSS that happened to have used a non-default
	// namespace.
	//
	// The one exception is that if we find wildcards namespaces we "upgrade"
	// them to "default" if there isn't already an existing intention.
	//
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
		output      structs.Intentions
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
			output = append(output, ixn)
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
			output = append(output, updated)
		}
	}

	return structs.MigrateIntentions(output)
}
