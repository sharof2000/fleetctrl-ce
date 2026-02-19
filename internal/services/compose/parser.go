package compose

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"fleetctrl/internal/models"
)

// Regex patterns for environment variable substitution
var (
	// Matches ${VAR} or ${VAR:-default}
	envVarBracesPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)
	// Matches $VAR (simple format, word boundary)
	envVarSimplePattern = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
)

// substituteEnvVars replaces environment variable references with their values
// Handles patterns: ${VAR}, ${VAR:-default}, $VAR, and $$ (escaped $)
func substituteEnvVars(value string, env map[string]string) string {
	if env == nil || !strings.Contains(value, "$") {
		return value
	}

	// First, handle escaped $$ -> $ (use a placeholder)
	const placeholder = "\x00DOLLAR\x00"
	result := strings.ReplaceAll(value, "$$", placeholder)

	// Handle ${VAR:-default} and ${VAR} patterns
	result = envVarBracesPattern.ReplaceAllStringFunc(result, func(match string) string {
		submatches := envVarBracesPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		varName := strings.TrimSpace(submatches[1])
		defaultVal := ""
		if len(submatches) >= 3 {
			defaultVal = submatches[2]
		}

		if val, ok := env[varName]; ok {
			return val
		}
		if defaultVal != "" {
			return defaultVal
		}
		// Variable not found and no default - keep original
		return match
	})

	// Handle $VAR patterns (only if not already substituted by braces pattern)
	result = envVarSimplePattern.ReplaceAllStringFunc(result, func(match string) string {
		submatches := envVarSimplePattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		varName := submatches[1]
		if val, ok := env[varName]; ok {
			return val
		}
		// Variable not found - keep original
		return match
	})

	// Restore escaped $$ -> $
	result = strings.ReplaceAll(result, placeholder, "$")

	return result
}

// ComposeSpec represents the parsed docker-compose.yml structure
type ComposeSpec struct {
	Version  string                 `yaml:"version"`
	Services map[string]ServiceSpec `yaml:"services"`
}

// ServiceSpec represents a single service definition in docker-compose
type ServiceSpec struct {
	Image         string        `yaml:"image"`
	Build         interface{}   `yaml:"build"` // can be string or struct
	ContainerName string        `yaml:"container_name"`
	Ports         []interface{} `yaml:"ports"` // handles various port formats
}

// ParseFile parses a docker-compose file and returns the service definitions
func ParseFile(filePath string) (*ComposeSpec, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}
	return Parse(data)
}

// Parse parses compose YAML content
func Parse(data []byte) (*ComposeSpec, error) {
	var spec ComposeSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse compose file: %w", err)
	}
	return &spec, nil
}

// ExtractServices converts ServiceSpec map to models.Service slice
// If env is provided, environment variables in values will be substituted
func (spec *ComposeSpec) ExtractServices(env map[string]string) []models.Service {
	services := make([]models.Service, 0, len(spec.Services))
	for name, svc := range spec.Services {
		// Apply env substitution to image and container name
		image := substituteEnvVars(svc.Image, env)
		containerName := substituteEnvVars(svc.ContainerName, env)

		service := models.Service{
			Name:          name,
			Image:         image,
			ContainerName: containerName,
			Ports:         parsePortsWithEnv(svc.Ports, env),
			Status:        models.ServiceStatusNotStarted,
			Source:        "compose",
		}

		// Handle build-only services (no image specified)
		if service.Image == "" && svc.Build != nil {
			service.Image = "[build]"
		}

		services = append(services, service)
	}

	// Sort services by name for consistent ordering
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return services
}

// parsePortsWithEnv handles various port formats and applies env substitution
func parsePortsWithEnv(ports []interface{}, env map[string]string) []string {
	result := make([]string, 0, len(ports))
	for _, p := range ports {
		switch v := p.(type) {
		case string:
			// Apply env substitution to port string
			substituted := substituteEnvVars(v, env)
			result = append(result, normalizePort(substituted))
		case int:
			result = append(result, fmt.Sprintf("%d:%d", v, v))
		case float64:
			// YAML can parse numbers as float64
			intVal := int(v)
			result = append(result, fmt.Sprintf("%d:%d", intVal, intVal))
		case map[string]interface{}:
			// Long format: {target: 80, published: 8080}
			if target, ok := v["target"]; ok {
				published := v["published"]
				if published == nil {
					published = target
				}
				// Handle env vars in long format values
				targetStr := fmt.Sprintf("%v", target)
				publishedStr := fmt.Sprintf("%v", published)
				targetStr = substituteEnvVars(targetStr, env)
				publishedStr = substituteEnvVars(publishedStr, env)
				result = append(result, fmt.Sprintf("%s:%s", publishedStr, targetStr))
			}
		}
	}
	return result
}

// normalizePort normalizes port strings to "host:container" format
func normalizePort(port string) string {
	// Remove protocol suffix if present (e.g., "80:80/tcp")
	if idx := strings.Index(port, "/"); idx != -1 {
		port = port[:idx]
	}

	// Handle formats: "80", "80:80", "127.0.0.1:80:80", "80-90:80-90"
	parts := strings.Split(port, ":")
	switch len(parts) {
	case 1:
		return fmt.Sprintf("%s:%s", parts[0], parts[0])
	case 2:
		return port
	case 3:
		// IP:hostPort:containerPort - strip IP for display
		return fmt.Sprintf("%s:%s", parts[1], parts[2])
	default:
		return port
	}
}
