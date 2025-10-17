package model

import (
	"fmt"
	"runtime"

	"github.com/urfave/cli/v3"
)

// VanityConfig holds the configuration for vanity address generation
type VanityConfig struct {
	Contains      string
	Mode          int  // 1: prefix, 2: suffix, 3: prefix-or-suffix
	CaseSensitive bool
	UpperOrLower  bool
	Threads       int
}

// VanityFlags returns the common CLI flags for vanity address generation
func VanityFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "contains",
			Aliases:  []string{"c"},
			Usage:    "The address must contain this string.",
			Required: true,
		},
		&cli.StringFlag{
			Name:    "mode",
			Aliases: []string{"m"},
			Usage:   "Matching position: prefix, suffix, prefix-or-suffix (default: prefix-or-suffix).",
			Value:   "prefix-or-suffix",
		},
		&cli.StringFlag{
			Name:    "case",
			Usage:   "Case matching mode: sensitive (default), insensitive, either.",
			Value:   "sensitive",
		},
		&cli.StringFlag{
			Name:    "threads",
			Aliases: []string{"t"},
			Usage:   "Number of threads to use (default: 1, use 'auto' for CPU cores).",
			Value:   "1",
		},
	}
}

// VanityCreate2Flags returns CLI flags for CREATE2 vanity address generation
func VanityCreate2Flags() []cli.Flag {
	flags := VanityFlags()
	// Add CREATE2 specific flags
	create2Flags := []cli.Flag{
		&cli.StringFlag{
			Name:     "deployer",
			Aliases:  []string{"d"},
			Usage:    "The deployer address.",
			Required: true,
		},
		&cli.StringFlag{
			Name:    "salt-prefix",
			Aliases: []string{"sp"},
			Usage:   "The prefix of the salt. keccak256(salt-prefix + randSaltSuffix)",
			Value:   "",
		},
		&cli.StringFlag{
			Name:     "contract-bin",
			Aliases:  []string{"cb"},
			Usage:    "The contract bytecode.",
			Required: true,
		},
		&cli.StringSliceFlag{
			Name:    "constructor-args",
			Aliases: []string{"ca"},
			Usage:   "The constructor arguments. Format: type:value (e.g., uint256:123, address:0x...)",
		},
	}
	return append(flags, create2Flags...)
}

// ParseVanityConfig parses the vanity configuration from CLI command
func ParseVanityConfig(cmd *cli.Command) (*VanityConfig, error) {
	contains := cmd.String("contains")
	modeStr := cmd.String("mode")
	caseStr := cmd.String("case")
	threadsStr := cmd.String("threads")

	if contains == "" {
		return nil, fmt.Errorf("contains cannot be empty")
	}

	// Parse mode
	var mode int
	switch modeStr {
	case "prefix":
		mode = 1
	case "suffix":
		mode = 2
	case "prefix-or-suffix":
		mode = 3
	default:
		return nil, fmt.Errorf("mode must be one of: prefix, suffix, prefix-or-suffix")
	}

	// Parse case matching
	var caseSensitive, upperOrLower bool
	switch caseStr {
	case "sensitive":
		caseSensitive = true
		upperOrLower = false
	case "insensitive":
		caseSensitive = false
		upperOrLower = false
	case "either":
		caseSensitive = true // We need to check exact case
		upperOrLower = true
	default:
		return nil, fmt.Errorf("case must be one of: sensitive, insensitive, either")
	}

	// Parse threads
	var threads int
	if threadsStr == "auto" {
		threads = runtime.NumCPU()
	} else {
		// Parse threads string to int
		var err error
		if _, err = fmt.Sscanf(threadsStr, "%d", &threads); err != nil || threads < 1 {
			return nil, fmt.Errorf("threads must be a positive integer or 'auto'")
		}
	}

	return &VanityConfig{
		Contains:      contains,
		Mode:          mode,
		CaseSensitive: caseSensitive,
		UpperOrLower:  upperOrLower,
		Threads:       threads,
	}, nil
}
