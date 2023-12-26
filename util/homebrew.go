package util

import "os/exec"

// InstalledWithHomebrew tries to determine if the cli was installed with homebrew
func InstalledWithHomebrew() bool {
	// Check if homebrew is installed
	cmd := exec.Command("brew", "info", "wercker-cli")
	err := cmd.Run()
	if err != nil {
		// if homebrew is not installed this will fail -> we know wercker was not installed with homebrew
		// if the brew command succeeds but the wercker-cli is not installed it will exit with code 1
		return false
	}

	return true
}
