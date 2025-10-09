package cmd

import (
	"io"
	"localapps-cli/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func init() {
	rootCmd.AddCommand(buildCmd)
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build app",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		currentDir, _ := os.Getwd()

		appFilePath := filepath.Join(currentDir, "app.yml")
		file, err := os.Open(appFilePath)

		if err != nil {
			cmd.PrintErrln("No app.yml file detected")
			return
		}
		defer file.Close()

		appFileContents, err := io.ReadAll(file)
		if err != nil {
			cmd.PrintErrf("failed to read app file: %v\n", err)
		}

		var app types.App
		err = yaml.Unmarshal(appFileContents, &app)
		if err != nil {
			cmd.PrintErrf("failed to parse app file: %v\n", err)
		}

		var appId string
		if app.Id != "" {
			appId = app.Id
		} else {
			appId = strings.ToLower(strings.ReplaceAll(app.Name, " ", "-"))
		}

		cmd.Println("Building " + app.Name)

		for partName, part := range app.Parts {
			buildCmd := exec.Command("docker", "build", "-t", "localapps/apps/"+appId+"/"+partName, part.Src)

			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr

			buildCmd.Run()
		}
	},
}
