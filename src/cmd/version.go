package cmd

import (
	"fmt"
	"localapps-cli/constants"
	"localapps-cli/utils"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays your CLI version",
	Run:   version,
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrades your CLI version",
	Run:   upgrade,
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.AddCommand(upgradeCmd)
}

func version(cmd *cobra.Command, args []string) {
	latestRelease, _ := utils.GetLatestCliVersion()

	if constants.Version < latestRelease.TagName {
		fmt.Println("A new update is avaliable")
		fmt.Println("Run 'localapps version upgrade' to upgrade")
	}
	fmt.Println("Your CLI Version:", constants.Version)
	fmt.Println("Latest CLI version:", latestRelease.TagName)
}

func upgrade(cmd *cobra.Command, args []string) {
	var command string
	var cmdArgs []string

	switch runtime.GOOS {
	case "linux", "darwin":
		command = "sh"
		cmdArgs = []string{"-c", "curl -fsSL https://raw.githubusercontent.com/CyberL1/localapps/main/scripts/get.sh | sh"}

	case "windows":
		command = "powershell"
		cmdArgs = []string{"irm https://raw.githubusercontent.com/CyberL1/localapps/main/scripts/get.ps1 | iex"}
	}

	execCmd := exec.Command(command, cmdArgs...)
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Run()
}
