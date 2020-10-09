package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func findSettingsDir() (string, error) {
	osSettingsMap := map[string][]string{
		"linux":  []string{os.Getenv("XDG_CONFIG_HOME"), filepath.Join(os.Getenv("HOME"), ".config")},
		"darwin": []string{filepath.Join(os.Getenv("HOME"), "Library")},
	}

	for _, dir := range osSettingsMap[runtime.GOOS] {
		if dir != "" {
			dir = filepath.Join(dir, "calwarrior")

			// not really sure where best to do this.
			if err := os.MkdirAll(dir, 0700); err != nil {
				return "", err
			}
			return dir, nil
		}
	}

	return "", errors.New("cannot find the appropriate dir to set configuration; try setting $HOME, container nerd")
}

func findLauncher() (string, bool) {
	osToolMap := map[string]string{
		"linux":  "xdg-open",
		"darwin": "open",
	}

	tool := osToolMap[runtime.GOOS]
	if s, err := exec.LookPath(tool); err == nil {
		return s, true
	}

	return "", false
}
