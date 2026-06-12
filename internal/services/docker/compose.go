package docker

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// ComposeUp runs docker compose up in the specified directory
func (s *Service) ComposeUp(applicationPath string) error {
	return s.runCompose(applicationPath, "", "up", "-d")
}

// ComposeUpFile runs docker compose up with a specific compose file
func (s *Service) ComposeUpFile(appPath, composeFile string) error {
	return s.runCompose(appPath, composeFile, "up", "-d")
}

// ComposeDown runs docker compose down in the specified directory
func (s *Service) ComposeDown(applicationPath string) error {
	return s.runCompose(applicationPath, "", "down")
}

// ComposeDownFile runs docker compose down with a specific compose file
func (s *Service) ComposeDownFile(appPath, composeFile string) error {
	return s.runCompose(appPath, composeFile, "down")
}

// ComposeStop runs docker compose stop in the specified directory
func (s *Service) ComposeStop(applicationPath string) error {
	return s.runCompose(applicationPath, "", "stop")
}

// ComposeStopFile runs docker compose stop with a specific compose file
func (s *Service) ComposeStopFile(appPath, composeFile string) error {
	return s.runCompose(appPath, composeFile, "stop")
}

// ComposeStart runs docker compose start in the specified directory
func (s *Service) ComposeStart(applicationPath string) error {
	return s.runCompose(applicationPath, "", "start")
}

// ComposeStartFile runs docker compose start with a specific compose file
func (s *Service) ComposeStartFile(appPath, composeFile string) error {
	return s.runCompose(appPath, composeFile, "start")
}

// ComposeRestart runs docker compose restart in the specified directory
func (s *Service) ComposeRestart(applicationPath string) error {
	return s.runCompose(applicationPath, "", "restart")
}

// ComposeRestartFile pulls the latest images then runs `up -d` so that any
// image-tag or env changes since the last start take effect. Plain
// `docker compose restart` would only bounce existing containers.
func (s *Service) ComposeRestartFile(appPath, composeFile string) error {
	if err := s.runCompose(appPath, composeFile, "pull"); err != nil {
		return err
	}
	return s.runCompose(appPath, composeFile, "up", "-d")
}

// ComposePull runs docker compose pull in the specified directory
func (s *Service) ComposePull(applicationPath string) error {
	return s.runCompose(applicationPath, "", "pull")
}

// ComposePullFile runs docker compose pull with a specific compose file
func (s *Service) ComposePullFile(appPath, composeFile string) error {
	return s.runCompose(appPath, composeFile, "pull")
}

// ComposePS returns the status of services in the stack
func (s *Service) ComposePS(applicationPath string) (string, error) {
	if err := s.checkAvailable(); err != nil {
		return "", err
	}

	cmd := exec.Command("docker", "compose", "ps", "--format", "json")
	cmd.Dir = applicationPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker compose ps failed: %w", err)
	}

	return string(output), nil
}

// ComposeUpService starts a specific service within a compose file
func (s *Service) ComposeUpService(appPath, composeFile, serviceName string) error {
	return s.runCompose(appPath, composeFile, "up", "-d", serviceName)
}

// ComposeStopService stops a specific service within a compose file
func (s *Service) ComposeStopService(appPath, composeFile, serviceName string) error {
	return s.runCompose(appPath, composeFile, "stop", serviceName)
}

// ComposeRestartService restarts a specific service within a compose file
func (s *Service) ComposeRestartService(appPath, composeFile, serviceName string) error {
	return s.runCompose(appPath, composeFile, "restart", serviceName)
}

func (s *Service) runCompose(applicationPath, composeFile string, args ...string) error {
	// Check if Docker is available (even though we use CLI, still need Docker running)
	if err := s.checkAvailable(); err != nil {
		return err
	}

	var fullArgs []string

	if composeFile != "" {
		// Use specific compose file
		fullArgs = append([]string{"compose", "-f", composeFile}, args...)
	} else {
		// Use default compose file(s) in directory
		fullArgs = append([]string{"compose"}, args...)
	}

	// Add --env-file flag if .env file exists to ensure compose reads fresh values
	envPath := filepath.Join(applicationPath, ".env")
	if _, err := os.Stat(envPath); err == nil {
		// Insert --env-file after compose command
		fullArgs = append([]string{fullArgs[0], "--env-file", ".env"}, fullArgs[1:]...)
	}

	cmd := exec.Command("docker", fullArgs...)
	cmd.Dir = filepath.Clean(applicationPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		cmdStr := args[0]
		if len(args) == 0 {
			cmdStr = "unknown"
		}
		return fmt.Errorf("docker compose %s failed: %w\nOutput: %s", cmdStr, err, string(output))
	}

	return nil
}

// RunComposeWithOutput runs a docker compose command and streams output via callback.
// action can be: up, stop, restart, down, pull. "restart" is special-cased to run
// `pull` followed by `up -d` so image-tag and env changes are picked up.
func (s *Service) RunComposeWithOutput(appPath, composeFile, action string, outputFn func(line string)) error {
	if err := s.checkAvailable(); err != nil {
		return err
	}

	if action == "restart" {
		if err := s.runComposeStreamed(appPath, composeFile, outputFn, "pull"); err != nil {
			return err
		}
		return s.runComposeStreamed(appPath, composeFile, outputFn, "up", "-d")
	}

	args := []string{action}
	if action == "up" {
		args = append(args, "-d")
	}
	return s.runComposeStreamed(appPath, composeFile, outputFn, args...)
}

// runComposeStreamed executes a single `docker compose ...` invocation and
// streams stdout+stderr line-by-line through outputFn. It injects --env-file .env
// when present so fresh env values are read on every call.
func (s *Service) runComposeStreamed(appPath, composeFile string, outputFn func(line string), args ...string) error {
	var fullArgs []string

	// Build the compose command
	if composeFile != "" {
		fullArgs = append([]string{"compose", "-f", composeFile}, args...)
	} else {
		fullArgs = append([]string{"compose"}, args...)
	}

	// Add --env-file flag if .env file exists to ensure compose reads fresh values
	envPath := filepath.Join(appPath, ".env")
	if _, err := os.Stat(envPath); err == nil {
		fullArgs = append([]string{fullArgs[0], "--env-file", ".env"}, fullArgs[1:]...)
	}

	cmd := exec.Command("docker", fullArgs...)
	cmd.Dir = filepath.Clean(appPath)

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Send initial command info
	outputFn(fmt.Sprintf("$ docker %s", joinArgs(fullArgs)))

	// Use WaitGroup to wait for both readers
	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout
	go func() {
		defer wg.Done()
		scanAndSend(stdout, outputFn)
	}()

	// Stream stderr
	go func() {
		defer wg.Done()
		scanAndSend(stderr, outputFn)
	}()

	// Wait for output readers to finish
	wg.Wait()

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		outputFn(fmt.Sprintf("Error: %v", err))
		return err
	}

	return nil
}

// scanAndSend reads from a reader and sends each line via callback
func scanAndSend(r io.Reader, outputFn func(line string)) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		outputFn(scanner.Text())
	}
}

// joinArgs joins command arguments for display
func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}
