package types

// Contains definition of policies to rename nodes

const (
	// NodeRenamingLegacy Legacy mode for creating a new node name or renaming - allow empty IDs
	NodeRenamingLegacy = "legacy"
	// NodeRenamingRenameDeadNodes Allow take name/renaming if node we want to steal the name is dead
	NodeRenamingRenameDeadNodes = "dead"
	// NodeRenamingStrict Allow take name/renaming if no similar node exists in catalog
	NodeRenamingStrict = "strict"
	// NodeRenamingDefault is the value set when nothing is set in configuration
	NodeRenamingDefault = NodeRenamingLegacy
)
