package cmd

import (
	"encoding/json"
	"io"

	"github.com/drewstinnett/gout/v2"
	"github.com/drewstinnett/gout/v2/formats"
	goutjson "github.com/drewstinnett/gout/v2/formats/json"
	"github.com/drewstinnett/gout/v2/formats/plain"
	"github.com/drewstinnett/gout/v2/formats/yaml"
	"github.com/spf13/cobra"
)

// outputConfig holds output formatting configuration.
type outputConfig struct {
	Format string
}

var outCfg = &outputConfig{}

// BindOutputFlags adds --format flag to a command.
// This should be called on the root command.
func BindOutputFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&outCfg.Format, "format", "plain", "Output format: plain, json, yaml, jsonl")
}

// PrintOutput prints data in the configured format.
func PrintOutput(w io.Writer, data interface{}) error {
	// Handle JSONL specially since gout doesn't have it built-in
	if outCfg.Format == "jsonl" {
		return printJSONL(w, data)
	}

	// Use gout for standard formats
	g := gout.New(gout.WithWriter(w))

	// Set format
	var formatter formats.Formatter
	switch outCfg.Format {
	case "json":
		formatter = &goutjson.Formatter{}
	case "yaml":
		formatter = &yaml.Formatter{}
	case "plain":
		formatter = &plain.Formatter{}
	default:
		formatter = &plain.Formatter{}
	}

	g.SetFormatter(formatter)
	return g.Print(data)
}

// printJSONL prints data as a JSON line (for streaming).
func printJSONL(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	return enc.Encode(data)
}

// IsStructuredOutput returns true if the output format is structured (JSON, YAML, etc.)
func IsStructuredOutput() bool {
	switch outCfg.Format {
	case "json", "yaml", "toml", "jsonl":
		return true
	default:
		return false
	}
}

// GetFormat returns the current output format.
func GetFormat() string {
	return outCfg.Format
}
