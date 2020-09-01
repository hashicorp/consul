package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	flags, opts := setupFlags(args[0])
	err := flags.Parse(os.Args[1:])
	switch {
	case err == flag.ErrHelp:
		flags.Usage()
		return nil
	case err != nil:
		return err
	}
	return runMog(*opts)
}

type options struct {
	source string
}

func setupFlags(name string) (*flag.FlagSet, *options) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	opts := &options{}

	flags.StringVar(&opts.source, "source", ".", "package path for source structs")
	return flags, opts
}

func runMog(opts options) error {
	sources, err := loadSourceStructs(opts.source)
	if err != nil {
		return fmt.Errorf("failed to load source from %s: %w", opts.source, err)
	}

	cfg, err := configsFromAnnotations(sources)
	if err != nil {
		return fmt.Errorf("failed to parse annotations: %w", err)
	}

	targets, err := loadTargetStructs(targetPackages(cfg.structs))
	if err != nil {
		return fmt.Errorf("failed to load targets: %w", err)
	}

	return generate(cfg, targets)
}

func targetPackages(cfgs []structConfig) []string {
	result := make([]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		if cfg.Target.Package == "" {
			continue
		}
		result = append(result, cfg.Target.Package)
	}
	return result
}
