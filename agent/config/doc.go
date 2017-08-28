// Package config contains the command line and config file code for the
// consul agent.
//
// The consul agent configuration is generated from multiple sources:
//
//  * config files
//  * environment variables (which?)
//  * cmd line args
//
// Each of these argument sets needs to be parsed, validated and then
// merged with the other sources to build the final configuration.
//
// This patch introduces a distinction between the user and the runtime
// configuration. The user configuration defines the external interface for
// the user, i.e. the command line flags, the environment variables and the
// config file format which cannot be changed without breaking the users'
// setup.
//
// The runtime configuration is the merged, validated and mangled
// configuration structure suitable for the consul agent. Both structures
// are similar but different and the runtime configuration can be
// refactored at will without affecting the user configuration format.
//
// For this, the user configuration consists of several structures for
// config files and command line arguments. Again, the config file and
// command line structs are similar but not identical for historical
// reasons and to allow evolving them differently.
//
// All of the user configuration structs have pointer values to
// unambiguously merge values from several sources into the final value.
//
// The runtime configuration has no pointer values and should be passed by
// value to avoid accidental or malicious runtime configuration changes.
// Runtime updates need to be handled through a new configuration
// instances.
//
// This code is work in progress and will first attempt to cover all edge
// cases before building the fully fledged configuration.
package config

// func doit() (c RuntimeConfig, warns []string, err error) {
// 	var flags Flags
// 	fs := flag.NewFlagSet("", flag.ContinueOnError)
// 	AddFlags(fs, &flags)
// 	if err = fs.Parse(os.Args); err != nil {
// 		return
// 	}
//
// 	b, err := NewBuilder(flags, defaultConfig)
// 	if err != nil {
// 		return RuntimeConfig{}, nil, err
// 	}
// 	for _, name := range flags.ConfigFiles {
// 		if err := b.readFile(name); err != nil {
// 			return RuntimeConfig{}, nil, err
// 		}
// 	}
// 	return b.Build()
// }
