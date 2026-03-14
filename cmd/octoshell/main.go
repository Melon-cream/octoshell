package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"octoshell/internal/octopus"
)

type config struct {
	accountNumber string
	propertyID    string
	timezone      string
	endpoint      string
	format        string
	showVersion   bool
	verbose       bool
	token         string
	email         string
	password      string
}

var newClient = octopus.NewClient
var version = "dev"

func main() {
	if err := run(context.Background(), os.Stdout, os.Stderr, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, stdout, stderr io.Writer, args []string) error {
	cfg, err := parseFlags(stderr, args)
	if err != nil {
		return err
	}
	if cfg.showVersion {
		_, err := fmt.Fprintln(stdout, version)
		return err
	}

	client := newClient(cfg.endpoint, nil)
	if cfg.verbose {
		client.SetDebugWriter(stderr)
	}
	token := cfg.token
	if token == "" {
		token, err = client.ObtainToken(ctx, cfg.email, cfg.password)
		if err != nil {
			return err
		}
	}

	usages, err := client.MonthlyElectricityUsage(ctx, token, octopus.UsageParams{
		AccountNumber: cfg.accountNumber,
		PropertyID:    cfg.propertyID,
		Timezone:      cfg.timezone,
	})
	if err != nil {
		return err
	}

	switch cfg.format {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(usages)
	case "csv":
		return octopus.WriteCSV(stdout, usages)
	default:
		return fmt.Errorf("unsupported format: %s", cfg.format)
	}
}

func parseFlags(stderr io.Writer, args []string) (config, error) {
	fs := flag.NewFlagSet("octoshell", flag.ContinueOnError)
	fs.SetOutput(stderr)

	cfg := config{}
	fs.StringVar(&cfg.accountNumber, "account-number", "", "Octopus account number")
	fs.StringVar(&cfg.propertyID, "property-id", "", "Property ID to query")
	fs.StringVar(&cfg.timezone, "timezone", "Asia/Tokyo", "Timezone for monthly aggregation")
	fs.StringVar(&cfg.endpoint, "endpoint", octopus.DefaultEndpoint, "GraphQL endpoint")
	fs.StringVar(&cfg.format, "format", "json", "Output format: json or csv")
	fs.BoolVar(&cfg.showVersion, "version", false, "Print version and exit")
	fs.BoolVar(&cfg.verbose, "v", false, "Verbose output to stderr")
	fs.StringVar(&cfg.token, "token", "", "Kraken token to use as Authorization header")
	fs.StringVar(&cfg.email, "email", "", "Email for obtainKrakenToken")
	fs.StringVar(&cfg.password, "password", "", "Password for obtainKrakenToken")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	cfg.accountNumber = strings.TrimSpace(cfg.accountNumber)
	cfg.propertyID = strings.TrimSpace(cfg.propertyID)
	cfg.timezone = strings.TrimSpace(cfg.timezone)
	cfg.endpoint = strings.TrimSpace(cfg.endpoint)
	cfg.format = strings.ToLower(strings.TrimSpace(cfg.format))
	cfg.token = strings.TrimSpace(cfg.token)
	cfg.email = strings.TrimSpace(cfg.email)

	if cfg.showVersion {
		return cfg, nil
	}

	if cfg.accountNumber == "" {
		return cfg, octopus.ErrMissingAccount
	}
	if cfg.format != "json" && cfg.format != "csv" {
		return cfg, fmt.Errorf("format must be json or csv")
	}

	if cfg.token == "" {
		if cfg.email == "" || cfg.password == "" {
			return cfg, errors.New("token or email/password is required")
		}
	}

	return cfg, nil
}
