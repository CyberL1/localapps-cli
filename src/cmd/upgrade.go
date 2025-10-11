package cmd

import (
	"errors"
	"fmt"
	"io"
	"localapps-cli/constants"
	"localapps-cli/utils"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Masterminds/semver"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the CLI",
	Run: func(cmd *cobra.Command, args []string) {
		latestRelease, err := utils.GetLatestCliVersion()
		if err != nil {
			fmt.Println("Could not get latest release:", err)
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

		if currentVersion.Equal(newVersion) || currentVersion.GreaterThan(newVersion) {
			fmt.Println("You're already at the latest version, no need to upgrade")
			return
		}

		assetName := buildAssetName()
		downloadURL := fmt.Sprintf("https://github.com/CyberL1/localapps-cli/releases/latest/download/%s", assetName)

		tmpFile := filepath.Join(os.TempDir(), assetName)
		fmt.Println("Downloading new version...")
		err = downloadFile(downloadURL, tmpFile)
		if err != nil {
			fmt.Println("Error downloading binary:", err)
			return
		}

		currentExe, err := os.Executable()
		if err != nil {
			fmt.Println("Could not determine current executable path:", err)
			return
		}

		backupExe := currentExe + ".bak"
		_ = os.Remove(backupExe)
		err = os.Rename(currentExe, backupExe)
		if err != nil {
			fmt.Println("Error backing up current binary:", err)
			return
		}

		err = os.Rename(tmpFile, currentExe)
		if err != nil {
			fmt.Println("Error replacing current binary:", err)
			_ = os.Rename(backupExe, currentExe)
			return
		}

		err = os.Chmod(currentExe, 0755)
		if err != nil {
			fmt.Println("Warning: could not set executable permissions:", err)
		}

		os.Remove(backupExe)
		fmt.Println("Upgrade complete.")
	},
}

// buildAssetName returns the correct binary name for current OS/ARCH
func buildAssetName() string {
	// Adjust this based on how you name your binaries in GitHub Releases
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("localapps-cli-%s-%s%s", runtime.GOOS, runtime.GOARCH, ext)
}

// downloadFile downloads a file from the given URL to the target path
func downloadFile(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("download failed: " + resp.Status)
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
