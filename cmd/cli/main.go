package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"goproxy/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		printUsage()
		return nil
	}

	configPath := "config/config.yaml"
	if len(args) >= 2 && args[0] == "--config" {
		configPath = args[1]
		args = args[2:]
	}
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "routes":
		return routes(configPath, args[1:])
	case "keys":
		return keys(configPath, args[1:])
	case "stats":
		fmt.Println("stats are available from the gateway at /analytics/routes when analytics.enabled is true")
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func routes(configPath string, args []string) error {
	if len(args) != 1 || args[0] != "list" {
		return fmt.Errorf("usage: gateway routes list")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	for _, route := range cfg.Routes {
		methods := "*"
		if len(route.Methods) > 0 {
			methods = strings.Join(route.Methods, ",")
		}
		fmt.Printf("%s\t%s\t%s\t%s\n", route.ID, route.PathPrefix, methods, route.Upstream)
	}
	return nil
}

func keys(configPath string, args []string) error {
	if len(args) == 1 && args[0] == "list" {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		for _, key := range cfg.APIKeys {
			fmt.Printf("%s\t%s\n", key.ID, key.Tenant)
		}
		return nil
	}

	if len(args) == 4 && args[0] == "create" {
		id := strings.TrimSpace(args[1])
		tenant := strings.TrimSpace(args[3])
		if args[2] != "--tenant" || id == "" || tenant == "" {
			return fmt.Errorf("usage: gateway keys create <id> --tenant <tenant>")
		}

		key, err := randomKey()
		if err != nil {
			return err
		}
		content, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("read config: %w", err)
		}

		next := string(content)
		if !strings.HasSuffix(next, "\n") {
			next += "\n"
		}
		if !strings.Contains(next, "\napi_keys:") && !strings.HasPrefix(next, "api_keys:") {
			next += "\napi_keys:\n"
		}
		next += fmt.Sprintf("  - id: %q\n    key: %q\n    tenant: %q\n", id, key, tenant)
		if err := os.WriteFile(configPath, []byte(next), 0o600); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		fmt.Println(key)
		return nil
	}

	return fmt.Errorf("usage: gateway keys list | gateway keys create <id> --tenant <tenant>")
}

func randomKey() (string, error) {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return "gp_" + hex.EncodeToString(bytes[:]), nil
}

func printUsage() {
	fmt.Println(`usage:
  gateway routes list
  gateway keys list
  gateway keys create <id> --tenant <tenant>
  gateway stats

options:
  --config <path>  config file path (default config/config.yaml)`)
}
