package constants

import (
	"os"
	"path/filepath"
)

var configDir, _ = os.UserConfigDir()

var (
	LocalappsDir     = filepath.Join(configDir, "localapps")
	Version          string
	GithubReleaseUrl = "https://api.github.com/repos/CyberL1/localapps-cli/releases/latest"
)
