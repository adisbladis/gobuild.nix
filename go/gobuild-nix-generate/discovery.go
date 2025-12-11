package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

type goModDownload struct {
	Path     string
	Version  string
	Info     string
	GoMod    string
	Zip      string
	Dir      string
	Sum      string
	GoModSum string
}

func downloadModules(directory string, packages []string) ([]*goModDownload, error) {
	var downloads []*goModDownload

	cmd := exec.Command("go", append([]string{"mod", "download", "--json"}, packages...)...)
	cmd.Dir = directory
	stdout, err := cmd.Output()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to run 'go mod download --json: %s\n%s", exiterr, exiterr.Stderr)
		} else {
			return nil, fmt.Errorf("failed to run 'go mod download --json': %s", err)
		}
	}

	dec := json.NewDecoder(bytes.NewReader(stdout))
	for {
		var dl *goModDownload
		err := dec.Decode(&dl)
		if err == io.EOF {
			break
		} else {
			downloads = append(downloads, dl)
		}
	}

	return downloads, nil
}
