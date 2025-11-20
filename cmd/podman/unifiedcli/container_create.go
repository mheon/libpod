package unifiedcli

import (
	"fmt"

	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	"github.com/containers/podman/v5/pkg/systemd/quadlet"
	"github.com/spf13/pflag"
)

// A CLIOption is a single option on the Podman CLI.
// It may, optionally, have a corresponding Quadlet key.
// It has a retrieval function (to retrieve the value from parse CLI arguments)
// and two parsing functions (one to parse the CLI option, one to parse the Quadlet key's value).
type CLIOption[T any, P any] struct {
	// CLIOptionName is the full (long) name of the CLI argument.
	// Must be set.
	CLIOptionName string
	// QuadletKeyName is the name of the key in the quadlet file.
	// May be left unset (""), in which case this option has no Quadlet equivalent.
	QuadletKeyName string
	// QuadletGroupName is the name of the group in which Quadlet keys are contained.
	// For example, .container files store their container-specific keys in `[Container]`
	// and should use "container" as the value here.
	// Must be set if QuadletKeyName is set.
	QuadletGroupName string
	// CLIRetrieveFn retrieves the value of the flag from all CLI flags
	CLIRetrieveFn func(*pflag.FlagSet, string) *T
	// CLIParseFn takes the value from the CLI and the base value from the SpecGenerator,
	// parses the CLI option, and (if necessary) overlays it onto the default from the
	// SpecGenerator
	CLIParseFn func(T, P) (P, error)
	// SpecgenField retrieves a pointer to the specific field in the SpecGen.
	// This must be set.
	// MUST return a valid pointer to a field in the SpecGenerator.
	SpecgenField func(*specgen.SpecGenerator) *P
	// PodSpecgenField retrieves a pointer to the specific field in the PodSpecGen.
	// The function may be nil as some CLI options are container only and do not apply to pods.
	// If the function is not nil, it MUST return a valid pointer to a field in the PodSpecGenerator.
	PodSpecgenField func(*specgen.PodSpecGenerator) *P
	// QuadletParseFn parses a systemd unitfile, three strings (quadlet key and group names and CLI option name),
	// and a PodmanCmdline, parses the given Quadlet option, and places it into the PodmanCmdline.
	// Unused if QuadletKeyName is unset.
	QuadletParseFn func(parser.UnitFile, string, string, string, *quadlet.PodmanCmdline) error
}

// Validate validates that the option is safe to be used.
func (opt *CLIOption[_, _]) Validate() error {
	quadletOptsSet := 0
	if opt.QuadletKeyName != "" {
		quadletOptsSet += 1
	}
	if opt.QuadletGroupName != "" {
		quadletOptsSet += 1
	}
	if quadletOptsSet == 1 {
		return fmt.Errorf("if one of QuadletKeyName or QuadletGroupName are set, both much be set")
	}
	if quadletOptsSet == 2 && opt.QuadletParseFn == nil {
		return fmt.Errorf("if a quadlet option is provided, a parsing function must also be provided")
	}

	if opt.CLIOptionName == "" {
		return fmt.Errorf("must provide a CLI option name")
	}
	if opt.CLIParseFn == nil || opt.CLIRetrieveFn == nil || opt.SpecgenField == nil {
		return fmt.Errorf("must provide a CLI parsing function, retrieval function, and SpecGen field retrieval function")
	}

	return nil
}

// ParseRunCLI takes parsed command-line options and a SpecGen, parses the value
// of the given CLIOption, and places the result in the SpecGen.
func (opt *CLIOption[T, P]) ParseRunCLI(g *specgen.SpecGenerator, flags *pflag.FlagSet) error {
	value := opt.CLIRetrieveFn(flags, opt.CLIOptionName)
	if value == nil {
		return nil
	}
	dest := opt.SpecgenField(g)
	val, err := opt.CLIParseFn(*value, *dest)
	if err != nil {
		return err
	}
	*dest = val
	return nil
}

// ParsePodCreateCLI takes parsed command-line options and a PodSpecGen,
// parses the value of the given CLIOption, and places the result in the PodSpecGen
func (opt *CLIOption[T, P]) ParsePodCreateCLI(g *specgen.PodSpecGenerator, flags *pflag.FlagSet) error {
	if opt.PodSpecgenField == nil {
		return nil
	}
	value := opt.CLIRetrieveFn(flags, opt.CLIOptionName)
	if value == nil {
		return nil
	}
	dest := opt.PodSpecgenField(g)
	val, err := opt.CLIParseFn(*value, *dest)
	if err != nil {
		return err
	}
	*dest = val
	return nil
}

// ParseQuadlet takes the value(s) associated with the Quadlet key and produces
// Podman CLI options from them.
func (opt *CLIOption[T, _]) ParseQuadlet(unit parser.UnitFile, cmdline *quadlet.PodmanCmdline) error {
	if opt.QuadletKeyName == "" {
		return nil
	}
	return opt.QuadletParseFn(unit, opt.QuadletKeyName, opt.QuadletGroupName, opt.CLIOptionName, cmdline)
}

var (
	// Could use maps to make these sets, but I don't think that degree of
	// guaranteed safety is necessary... Worst case, we parse an option
	// twice.
	runStringOpts      []CLIOption[string, any]
	runStringArrayOpts []CLIOption[[]string, any]
)

// AddStringOption adds an option to parse a string field from the CLI
func AddStringOption(opt CLIOption[string, any]) error {
	if err := opt.Validate(); err != nil {
		return err
	}
	runStringOpts = append(runStringOpts, opt)
	return nil
}

// AddStringArrayOption adds an option to parse a string array field from the CLI
func AddStringArrayOption(opt CLIOption[[]string, any]) error {
	if err := opt.Validate(); err != nil {
		return err
	}
	runStringArrayOpts = append(runStringArrayOpts, opt)
	return nil
}

// Parse a complete `podman run` CLI from the given flagset
func ParseRunCLI(flags *pflag.FlagSet) (*specgen.SpecGenerator, error) {
	var g *specgen.SpecGenerator

	for _, o := range runStringOpts {
		if err := o.ParseRunCLI(g, flags); err != nil {
			return nil, err
		}
	}
	for _, o := range runStringArrayOpts {
		if err := o.ParseRunCLI(g, flags); err != nil {
			return nil, err
		}
	}

	return g, nil
}

// ParseQuadletFile takes a pre-parsed Quadlet file (parsed into a map of key
// to values) and parses it into `podman run` CLI options
func ParseQuadletFile(unit parser.UnitFile, cmdline *quadlet.PodmanCmdline) error {
	for _, o := range runStringOpts {
		err := o.ParseQuadlet(unit, cmdline)
		if err != nil {
			return err
		}
	}
	for _, o := range runStringOpts {
		err := o.ParseQuadlet(unit, cmdline)
		if err != nil {
			return err
		}
	}

	return nil
}
