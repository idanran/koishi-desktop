package main

import (
	"fmt"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

func legacyUserDataDir() (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, "Library/Application Support/Il Harper/Koishi"), nil
}
