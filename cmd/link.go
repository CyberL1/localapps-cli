package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"localapps-cli/utils"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(linkCmd)
}

var linkCmd = &cobra.Command{
	Use:   "link [url]",
	Short: "Allows you to link the CLI with the server.",
	Long:  "Allows you to link the CLI with the server. If no URL is provided, it will use saved url from config.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		urlToLink := utils.CliConfig.Server.Url
		if len(args) == 1 {
			urlToLink = args[0]
		}

		if !strings.HasPrefix(urlToLink, "http://") && !strings.HasPrefix(urlToLink, "https://") {
			urlToLink = "http://" + urlToLink
		}

		resp, err := http.Get(urlToLink + "/api/link")
		if err != nil {
			fmt.Printf("Failed to connect to server: %v\n", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			fmt.Printf("Server returned status: %s\n", resp.Status)
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Failed to read response body: %v\n", err)
			return
		}

		// Decode JSON response to get the API key
		var result struct {
			ApiKey string `json:"apiKey"`
		}
		err = json.Unmarshal(body, &result)
		if err != nil {
			fmt.Printf("Failed to decode response body: %v\n", err)
			return
		}

		// Save URL and key if successful link
		utils.CliConfig.Server.Url = urlToLink
		utils.CliConfig.Server.ApiKey = result.ApiKey

		err = utils.SaveCliConfig()
		if err != nil {
			fmt.Printf("Failed to update config: %v\n", err)
			return
		}
		fmt.Println("Linked succesfully")
	},
}
