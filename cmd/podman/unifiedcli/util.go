package unifiedcli

import (
	"fmt"
	"strings"

	"github.com/containers/podman/v5/pkg/systemd/parser"
	"github.com/containers/podman/v5/pkg/systemd/quadlet"
	"github.com/spf13/pflag"
)

// Helpers for CLIRetrieveFn

// RetrieveStringFromCLI is a function for retrieving a string, meant to be used with CLIRetrieveFn in the CLIOption[string] struct
func RetrieveStringFromCLI(flags *pflag.FlagSet, flagName string) *string {
	if flags.Changed(flagName) {
		val, err := flags.GetString(flagName)
		if err != nil {
			return nil
		}
		return &val
	}
	return nil
}

// RetrieveStringArrayFromCLI is a function for retrieving a string array, meant to be used with CLIRetrieveFn in the CLIOption[[]string] struct
func RetrieveStringArrayFromCLI(flags *pflag.FlagSet, flagName string) *[]string {
	if flags.Changed(flagName) {
		val, err := flags.GetStringArray(flagName)
		if err != nil {
			return nil
		}
		return &val
	}
	return nil
}

// RetrieveStringSliceFromCLI is a function for retrieving a string array, meant to be used with CLIRetrieveFn in the CLIOption[[]string] struct
func RetrieveStringSliceFromCLI(flags *pflag.FlagSet, flagName string) *[]string {
	if flags.Changed(flagName) {
		val, err := flags.GetStringSlice(flagName)
		if err != nil {
			return nil
		}
		return &val
	}
	return nil
}

// Sample CLI parsing functions

// ParseKVStringsOverDefaults parses =-separated KV strings into a map[string]string, overlaying over provided defaults.
func ParseKVStringsOverDefaults(kv []string, defaults map[string]string) (map[string]string, error) {
	for _, kvPair := range kv {
		kvSplit := strings.SplitN(kvPair, "=", 2)
		if len(kvSplit) != 2 {
			return nil, fmt.Errorf("must provide a key-value pair separated by =, instead got %s", kvPair)
		}
		defaults[kvSplit[0]] = kvSplit[1]
	}
	return defaults, nil
}

// Sample Quadlet parsing functions

// QuadletParseOptionBeforeEvery looks up every instance of a key and adds them all to the Podman command line.
// The CLI argument is prepended before each.
func QuadletParseOptionBeforeEvery(unit parser.UnitFile, key, group, podmanarg string, cmd *quadlet.PodmanCmdline) error {
	options := unit.LookupAll(group, key)
	for _, opt := range options {
		cmd.Args = append(cmd.Args, fmt.Sprintf("--%s", podmanarg), opt)
	}
	return nil
}
