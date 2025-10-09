package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"localapps-cli/constants"
	"localapps-cli/types"
	"localapps-cli/utils"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func init() {
	rootCmd.AddCommand(deployCmd)
}

var deployCmd = &cobra.Command{
	Use:     "deploy",
	Short:   "Deploy your app to the server",
	Args:    cobra.NoArgs,
	Aliases: []string{"push"},
	Run: func(cmd *cobra.Command, args []string) {
		err := uploadApp("app.yml", false)
		if err != nil {
			fmt.Println("Initial upload error:", err)
			return
		}
	},
}

func uploadApp(appFilePath string, update bool) error {
	appFile, err := os.Open(appFilePath)
	if err != nil {
		return fmt.Errorf("error opening file: %s", err)
	}
	defer appFile.Close()

	appFileContents, _ := io.ReadAll(appFile)

	var appInfo types.App
	if err := yaml.Unmarshal(appFileContents, &appInfo); err != nil {
		return fmt.Errorf("yaml parsing error: %w", err)
	}

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	appFormFile, err := writer.CreateFormFile("file", appFilePath)
	if err != nil {
		return fmt.Errorf("error creating form file: %s", err)
	}

	_, err = appFile.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("error resetting file position: %w", err)
	}

	_, err = io.Copy(appFormFile, appFile)
	if err != nil {
		return fmt.Errorf("error copying file: %s", err)
	}

	if appInfo.Icon != "" {
		iconFormFile, err := writer.CreateFormFile("icon", appInfo.Icon)
		if err != nil {
			return fmt.Errorf("error creating from file: %s", err)
		}

		iconFile, err := os.Open(appInfo.Icon)
		if err != nil {
			return fmt.Errorf("error opening file: %s", err)
		}
		defer iconFile.Close()

		_, err = io.Copy(iconFormFile, iconFile)
		if err != nil {
			return fmt.Errorf("error copying file: %s", err)
		}
	}

	if update {
		err = writer.WriteField("update", "true")
		if err != nil {
			return fmt.Errorf("error adding update field: %s", err)
		}
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("error closing writer: %s", err)
	}

	req, err := http.NewRequest("POST", utils.CliConfig.Server.Url+"/api/apps/", &requestBody)
	if err != nil {
		return fmt.Errorf("error creating request: %s", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", utils.CliConfig.Server.ApiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %s", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %s", err)
	}

	var bodyJson types.ApiError
	json.Unmarshal(body, &bodyJson)

	if bodyJson.Code == constants.ErrorAppInstalled && !update {
		return uploadApp(appFilePath, true)
	}

	if resp.StatusCode != http.StatusNoContent {
		fmt.Printf("[Error -> %s] %s\n\n", bodyJson.Code, bodyJson.Message)
		fmt.Println(bodyJson.Error.Error())
	} else {
		fmt.Println("App deployed. Find it on the server:", utils.CliConfig.Server.Url)
	}

	return nil
}
