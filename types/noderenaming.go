package types

import (
	"fmt"
)

// NodeRenamingPolicy Contains definition of policies to rename nodes
type NodeRenamingPolicy string

const (
	// NodeRenamingLegacy Legacy mode for creating a new node name or renaming - allow empty IDs
	NodeRenamingLegacy NodeRenamingPolicy = "legacy"
	// NodeRenamingRenameDeadNodes Allow take name/renaming if node we want to steal the name is dead
	NodeRenamingRenameDeadNodes NodeRenamingPolicy = "dead"
	// NodeRenamingStrict Allow take name/renaming if no similar node exists in catalog
	NodeRenamingStrict NodeRenamingPolicy = "strict"
	// NodeRenamingDefault is the value set when nothing is set in configuration
	NodeRenamingDefault = NodeRenamingLegacy
)

// ConvertToNodeRenamingLegacy Try converting a string into a NodeRenamingPolicy
func ConvertToNodeRenamingLegacy(nodeRenamingPolicy string) (NodeRenamingPolicy, error) {
	val := NodeRenamingPolicy(nodeRenamingPolicy)
	switch val {
	case "":
		return NodeRenamingPolicy(NodeRenamingDefault), nil
	case NodeRenamingLegacy:
	case NodeRenamingRenameDeadNodes:
	case NodeRenamingStrict:
	default:
		return NodeRenamingDefault, fmt.Errorf("Unsupported node_renaming_policy '%s', can be (%s|%s|%s)", nodeRenamingPolicy, NodeRenamingLegacy, NodeRenamingRenameDeadNodes, NodeRenamingStrict)
	}
	return val, nil
}
