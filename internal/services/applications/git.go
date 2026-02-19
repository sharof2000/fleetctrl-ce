package applications

import (
	"fmt"
	"os/exec"
	"regexp"
)

// validBranchName validates git branch names to prevent command injection
// Allows alphanumeric, hyphen, underscore, forward slash, and dot
var validBranchName = regexp.MustCompile(`^[a-zA-Z0-9_.\-/]+$`)

// gitClone clones a git repository to the specified path
func gitClone(repoURL, branch, destPath string) error {
	args := []string{"clone", "--depth", "1"}

	if branch != "" {
		// Validate branch name to prevent command injection
		if !validBranchName.MatchString(branch) {
			return fmt.Errorf("invalid branch name: contains disallowed characters")
		}
		args = append(args, "-b", branch)
	}

	args = append(args, repoURL, destPath)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// gitPull pulls the latest changes in a repository
func gitPull(repoPath string) error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// gitGetCurrentBranch returns the current branch name
func gitGetCurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get branch: %w", err)
	}

	return string(output), nil
}

// gitGetRemoteURL returns the remote origin URL
func gitGetRemoteURL(repoPath string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}

	return string(output), nil
}
