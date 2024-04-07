package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, solidityCmd)
}

var solidityCmd = &cli.Command{
	Name:  "solidity",
	Usage: "Enhanced solidity command.",
	Subcommands: []*cli.Command{
		unflattenCmd,
	},
}

var extractFileNamePattern = regexp.MustCompile(`.*\/([^\/]*\.sol)`)

var unflattenCmd = &cli.Command{
	Name:  "unflatten",
	Usage: "Unflatten a flattened solidity file.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "file",
			Usage: "The flattened solidity file.",
		},
		&cli.StringFlag{
			Name:  "output",
			Usage: "The output directory.",
			Value: ".",
		},
		&cli.BoolFlag{
			Name:    "keep-dependencies",
			Aliases: []string{"kd"},
			Usage:   "Keep dependencies.",
		},
	},
	Action: func(c *cli.Context) error {
		file := c.String("file")
		outputPath := c.String("output")
		keepDependencies := c.Bool("keep-dependencies")

		f, err := os.OpenFile(file, os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		fileInfo, err := f.Stat()
		if err != nil {
			return err
		}
		if fileInfo.Size() > 10*1024*1024 {
			return fmt.Errorf("file size is too large")
		}

		scanner := bufio.NewScanner(f)

		var filename, license, solcVersion string
		sb := new(strings.Builder)
		dependencies := make(map[string]string)

		writeContract := func(fName string, fContent string) error {
			fmt.Printf("Writing file %s %d\n", fName, sb.Len())

			dir := path.Dir(fName)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return err
				}
			}

			var dep strings.Builder
			for k, v := range dependencies {
				if strings.Contains(fContent, k) && !strings.HasSuffix(fName, k+".sol") {
					dep.WriteString(v)
				}
			}

			return os.WriteFile(fName, []byte(license+solcVersion+dep.String()+fContent), 0644)
		}

		genDependence := func(line string) string {
			filename = extractFileNamePattern.FindStringSubmatch(line)[1]
			filepath, _, _ := strings.Cut(line[9:], "@")
			filepath = string(line[8]) + filepath
			contractName, _ := strings.CutSuffix(filename, ".sol")
			dependencies[contractName] = fmt.Sprintf("import {%s} from \"%s\";\n", contractName, filepath)
			return filepath
		}

		var filenames []string
		var fileContents []string

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "import") && !keepDependencies {
				continue
			}

			if strings.HasPrefix(line, "// SPDX-License-Identifier:") {
				license = line + "\n\n"
				continue
			}

			if strings.HasPrefix(line, "pragma solidity") {
				solcVersion = line + "\n\n"
				continue
			}

			if strings.HasPrefix(line, "// File ") {
				if filename != "" {
					filenames = append(filenames, filename)
					fileContents = append(fileContents, sb.String())
				}
				sb.Reset()
				if line[8] == '@' && !keepDependencies {
					fmt.Printf("Skipping file %s\n", line[8:])
					genDependence(line)
					filename = ""
				} else {
					filename = genDependence(line)
					filename = outputPath + "/" + filename
				}
				continue
			}
			sb.WriteString(line)
			sb.Write([]byte("\n"))
		}

		if filename != "" {
			filenames = append(filenames, filename)
			fileContents = append(fileContents, sb.String())
		}

		for i := 0; i < len(filenames); i++ {
			if err := writeContract(filenames[i], fileContents[i]); err != nil {
				return err
			}
		}
		return nil
	},
}
