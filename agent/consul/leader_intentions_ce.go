//go:build !consulent
// +build !consulent

package consul

import (
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

func migrateIntentionsToConfigEntries(ixns structs.Intentions) []*structs.ServiceIntentionsConfigEntry {
	// Remove any intention in CE that happened to have used a non-default
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
			continue // a-ok for CE
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
		}
	}

	for name, updated := range tryUpgrades {
		// Check to see if the update we wanted to do would collide with an
		// existing intention. If so, we delete our original wildcard intention
		// via simply omitting it from migration.
		if _, collision := retained[name]; !collision {
			output = append(output, updated)
		}
	}

	return structs.MigrateIntentions(output)
}

func (s *Server) filterMigratedLegacyIntentions(entries []structs.ConfigEntry) ([]structs.ConfigEntry, error) {
	return entries, nil
}
