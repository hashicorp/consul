package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul-topology/sprawl"
	"github.com/hashicorp/consul-topology/topology"
)

const ProgramName = "consulcluster"

func main() {
	log.SetOutput(io.Discard)

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:       ProgramName,
		Level:      hclog.Debug,
		Output:     os.Stderr,
		JSONFormat: false,
	})

	if err := run(logger); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func run(logger hclog.Logger) error {
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg := SampleTopology1()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sp, err := sprawl.Launch(logger, workdir, cfg)
	if err != nil {
		return fmt.Errorf("error during launch: %w", err)
	}

	//nolint:errcheck
	defer sp.Stop()

	logger.Info("================ SLEEPING 5s BEFORE RELAUNCH ================")

	time.Sleep(5 * time.Second)

	node := cfg.Cluster("dc1").
		NodeByID(topology.NodeID{Name: "dc1-client2", Partition: "ap1"})
	node.Disabled = true

	logger.Info("================ RELAUNCHING to remove ap1/dc1-client2 ================")
	if err := sp.Relaunch(cfg); err != nil {
		return fmt.Errorf("error during relaunch: %w", err)
	}
	logger.Info("================ RELAUNCH IS COMPLETE; sleeping 5m for inspection ================")

	select {
	case <-time.After(5 * time.Minute):
	case sig := <-sigs:
		logger.Info("Caught signal; exiting", "signal", sig)
	}

	return nil
}
