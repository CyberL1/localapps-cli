package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"localapps-cli/types"
	"localapps-cli/utils"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
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
		cli, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

		_, err := cli.Ping(context.Background())
		if err != nil {
			fmt.Println("Failed to connect to Docker daemon. Is it running?")
			return
		}

		appFile, err := os.Open("app.yml")
		if err != nil {
			fmt.Printf("error opening file: %s", err)
			return
		}
		defer appFile.Close()

		appFileContents, _ := io.ReadAll(appFile)

		var appInfo types.App
		if err := yaml.Unmarshal(appFileContents, &appInfo); err != nil {
			fmt.Printf("yaml parsing error: %s", err)
			return
		}

		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		appFormFile, err := writer.CreateFormFile("file", "app.yml")
		if err != nil {
			fmt.Printf("error creating form file: %s", err)
			return
		}

		_, err = appFile.Seek(0, io.SeekStart)
		if err != nil {
			fmt.Printf("error resetting file position: %s", err)
			return
		}

		_, err = io.Copy(appFormFile, appFile)
		if err != nil {
			fmt.Printf("error copying file: %s", err)
			return
		}

		if appInfo.Icon != "" {
			iconFormFile, err := writer.CreateFormFile("icon", appInfo.Icon)
			if err != nil {
				fmt.Printf("error creating from file: %s", err)
				return
			}

			iconFile, err := os.Open(appInfo.Icon)
			if err != nil {
				fmt.Printf("error opening file: %s", err)
				return
			}
			defer iconFile.Close()

			_, err = io.Copy(iconFormFile, iconFile)
			if err != nil {
				fmt.Printf("error copying file: %s", err)
				return
			}
		}

		var appId string
		if appInfo.Id != "" {
			appId = appInfo.Id
		} else {
			appId = strings.ToLower(strings.ReplaceAll(appInfo.Name, " ", "-"))
		}

		// Set 'update' field if update needed
		appInfoReq, _ := http.NewRequest("GET", utils.CliConfig.Server.Url+"/api/apps/"+appId, nil)
		appInfoReq.Header.Set("Authorization", utils.CliConfig.Server.ApiKey)

		appInfoResp, err := http.DefaultClient.Do(appInfoReq)
		if err != nil {
			fmt.Println("Unable to reach the server")
			return
		}

		if appInfoResp.StatusCode == 200 {
			err = writer.WriteField("update", "true")
			if err != nil {
				fmt.Printf("error adding update field: %s", err)
				return
			}
		}

		err = writer.Close()
		if err != nil {
			fmt.Printf("error closing writer: %s", err)
			return
		}

		fmt.Println("Building " + appInfo.Name)

		for partName, part := range appInfo.Parts {
			buildCmd := exec.Command("docker", "build", "-t", "localapps/apps/"+appId+"/"+partName, part.Src)

			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr

			buildCmd.Run()
		}

		fmt.Println("Deploying app to the server")

		openRegistryReq, err := http.NewRequest("GET", utils.CliConfig.Server.Url+"/api/registry", nil)
		if err != nil {
			fmt.Printf("error creating request: %s", err)
			return
		}

		openRegistryReq.Header.Set("Authorization", utils.CliConfig.Server.ApiKey)

		openRegistryResp, err := http.DefaultClient.Do(openRegistryReq)
		if err != nil {
			fmt.Printf("error sending request: %s", err)
			return
		}
		defer openRegistryResp.Body.Close()

		body, err := io.ReadAll(openRegistryResp.Body)
		if err != nil {
			fmt.Printf("error reading response body: %s", err)
			return
		}

		var openRegistrybodyJson struct {
			Port string `json:"port"`
		}
		json.Unmarshal(body, &openRegistrybodyJson)

		serverUrlParsed, _ := url.Parse(utils.CliConfig.Server.Url)
		registryTarget, _ := url.Parse(openRegistryReq.URL.Scheme + "://" + serverUrlParsed.Hostname() + ":" + openRegistrybodyJson.Port)

		proxy := httputil.NewSingleHostReverseProxy(registryTarget)
		proxy.FlushInterval = -1

		proxyPort, _ := utils.GetFreePort()
		proxyPortString := strconv.Itoa(proxyPort)

		server := &http.Server{
			Addr:    "127.0.0.1:" + proxyPortString,
			Handler: proxy,
		}

		go func() {
			server.ListenAndServe()
		}()

		for partName := range appInfo.Parts {
			cli.ImageTag(context.Background(), "localapps/apps/"+appId+"/"+partName, "localhost:"+proxyPortString+"/localapps/apps/"+appId+"/"+partName)
			pushResp, err := cli.ImagePush(context.Background(), "localhost:"+proxyPortString+"/localapps/apps/"+appId+"/"+partName, image.PushOptions{})

			if err != nil {
				fmt.Printf("error pushing image: %s", err)
				return
			}
			defer pushResp.Close()

			if _, err := io.Copy(os.Stdout, pushResp); err != nil {
				fmt.Printf("error streaming push output: %s", err)
				return
			}
		}

		req, err := http.NewRequest("POST", utils.CliConfig.Server.Url+"/api/apps", &requestBody)
		if err != nil {
			fmt.Printf("error creating request: %s", err)
			return
		}

		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", utils.CliConfig.Server.ApiKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("error sending request: %s", err)
			return
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("error reading response body: %s", err)
			return
		}

		var bodyJson types.ApiError
		json.Unmarshal(body, &bodyJson)

		closeRegistryReq, err := http.NewRequest("DELETE", utils.CliConfig.Server.Url+"/api/registry", nil)
		if err != nil {
			fmt.Printf("error creating request: %s", err)
			return
		}

		closeRegistryReq.Header.Set("Authorization", utils.CliConfig.Server.ApiKey)

		_, err = http.DefaultClient.Do(closeRegistryReq)
		if err != nil {
			fmt.Printf("error sending request: %s", err)
			return
		}

		if resp.StatusCode != http.StatusNoContent {
			fmt.Printf("[Error -> %s] %s\n\n", bodyJson.Code, bodyJson.Message)
			fmt.Println(bodyJson.Error.Error())
		} else {
			appURL, _ := url.Parse(serverUrlParsed.Scheme + "://" + appId + "." + serverUrlParsed.Host)
			fmt.Println("\nApp deployed. It can be accessed under this URL:", appURL)
		}
	},
}
