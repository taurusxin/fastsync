package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/pflag"
	"github.com/taurusxin/fastsync/pkg/client"
	"github.com/taurusxin/fastsync/pkg/config"
	"github.com/taurusxin/fastsync/pkg/daemon"
	"github.com/taurusxin/fastsync/pkg/logger"
)

func main() {
	// 1. Try to parse as Daemon mode
	daemonFlags := pflag.NewFlagSet("daemon", pflag.ContinueOnError)
	daemonFlags.SetOutput(io.Discard) // Silence errors during speculative parse
	var configPath string
	daemonFlags.StringVarP(&configPath, "config", "c", "", "Config file path")

	// We speculate: if parsing succeeds, config is set, and NO extra args, it's daemon.
	err := daemonFlags.Parse(os.Args[1:])
	isDaemon := false
	if err == nil && configPath != "" && daemonFlags.NArg() == 0 {
		isDaemon = true
	}

	if isDaemon {
		// Run Daemon
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}

		// Setup Global Logger
		var logOut io.Writer = os.Stdout
		if cfg.LogFile != "" && cfg.LogFile != "stdout" {
			f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Printf("Failed to open log file: %v, using stdout\n", err)
			} else {
				logOut = f
			}
		}
		logger.SetGlobal(logger.New(logOut, logger.ParseLevel(cfg.LogLevel), "Main"))

		logger.Info("Starting FastSync Daemon...")
		daemon.Run(cfg)
		return
	}

	// 2. Normal Mode
	clientFlags := pflag.NewFlagSet("client", pflag.ExitOnError)
	var opts client.Options

	clientFlags.BoolVarP(&opts.Delete, "delete", "d", false, "Delete extraneous files from target")
	clientFlags.BoolVarP(&opts.Overwrite, "overwrite", "o", false, "Overwrite existing files")
	clientFlags.BoolVarP(&opts.Checksum, "checksum", "s", false, "Checksum check")
	clientFlags.BoolVarP(&opts.Compress, "compress", "z", false, "Compress file data during the transfer")
	clientFlags.BoolVarP(&opts.Archive, "archive", "a", false, "Archive mode")
	clientFlags.BoolVarP(&opts.Verbose, "verbose", "v", false, "Verbose output")

	clientFlags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [source] [target] [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s -c config.toml (Daemon Mode)\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		clientFlags.PrintDefaults()
	}

	clientFlags.Parse(os.Args[1:])

	args := clientFlags.Args()
	if len(args) < 2 {
		clientFlags.Usage()
		os.Exit(1)
	}

	source := args[0]
	target := args[1]

	// Setup Logger for Client
	// Always use Info level to show basic summary.
	// Detailed per-file logs will be controlled by opts.Verbose check in client code.
	logLevel := logger.LevelInfo
	logger.SetGlobal(logger.New(os.Stdout, logLevel, ""))

	client.Run(source, target, opts)
}
