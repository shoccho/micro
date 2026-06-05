package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type WorkspaceConfig struct {
	F5 string
	F6 string
	F7 string
}

func LoadWorkspaceConfig() *WorkspaceConfig {
	dir, err := os.Getwd()
	if err != nil {
		return nil
	}
	for {
		cfgPath := filepath.Join(dir, ".microrc.ini")
		f, err := os.Open(cfgPath)
		if err == nil {
			defer f.Close()
			cfg := &WorkspaceConfig{}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				switch key {
				case "f5":
					cfg.F5 = val
				case "f6":
					cfg.F6 = val
				case "f7":
					cfg.F7 = val
				}
			}
			return cfg
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

func (c *WorkspaceConfig) CommandFor(key string) string {
	switch key {
	case "F5":
		return c.F5
	case "F6":
		return c.F6
	case "F7":
		return c.F7
	}
	return ""
}
