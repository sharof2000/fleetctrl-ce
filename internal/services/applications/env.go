package applications

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

// writeEnvFile writes environment variables to a .env file
func writeEnvFile(filepath string, env map[string]string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Sort keys for consistent output
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := env[k]
		// Quote values containing spaces or special characters
		if strings.ContainsAny(v, " \t\n\"'$") {
			v = fmt.Sprintf("\"%s\"", strings.ReplaceAll(v, "\"", "\\\""))
		}
		if _, err := fmt.Fprintf(file, "%s=%s\n", k, v); err != nil {
			return err
		}
	}

	return nil
}

// readEnvFile reads environment variables from a .env file
func readEnvFile(filepath string) (map[string]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	defer file.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first =
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		env[key] = value
	}

	return env, scanner.Err()
}
