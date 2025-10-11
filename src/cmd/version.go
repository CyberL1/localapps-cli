package cmd

import (
	"fmt"
	"localapps-cli/constants"
	"localapps-cli/utils"

	"github.com/Masterminds/semver"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Check the CLI version",
	Run:   version,
}

func version(cmd *cobra.Command, args []string) {
	latestRelease, err := utils.GetLatestCliVersion()
	if err != nil {
		fmt.Println("Failed to get latest release", err)
		return
	}

	currentVersion, err := semver.NewVersion(constants.Version)
	if err != nil {
		fmt.Println("Failed to parse current version", err)
		return
	}

	newVersion, err := semver.NewVersion(latestRelease.TagName)
	if err != nil {
		fmt.Println("Failed to parse latest version", err)
		return
	}

	if currentVersion.LessThan(newVersion) {
		fmt.Println("A new update is available\nRun 'localapps-cli upgrade' to upgrade")
	}

	fmt.Printf("Your CLI Version: %s\nLatest CLI version: %s\n", constants.Version, latestRelease.TagName)
}
