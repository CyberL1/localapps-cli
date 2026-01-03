package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"localapps-cli/constants"
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
		err := uploadApp("app.yml", false)
		if err != nil {
			fmt.Println("Initial upload error:", err)
			return
		}
	},
}

func uploadApp(appFilePath string, update bool) error {
	cli, _ := client.NewClientWithOpts(client.FromEnv)

	_, err := cli.Ping(context.Background())
	if err != nil {
		fmt.Println("Failed to connect to Docker daemon. Is it running?")
		return err
	}

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

	fmt.Println("Building " + appInfo.Name)

	var appId string
	if appInfo.Id != "" {
		appId = appInfo.Id
	} else {
		appId = strings.ToLower(strings.ReplaceAll(appInfo.Name, " ", "-"))
	}

	for partName, part := range appInfo.Parts {
		buildCmd := exec.Command("docker", "build", "-t", "localapps/apps/"+appId+"/"+partName, part.Src)

		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr

		buildCmd.Run()
	}

	fmt.Println("Deploying app to the server")

	openRegistryReq, err := http.NewRequest("GET", utils.CliConfig.Server.Url+"/api/registry", nil)
	if err != nil {
		return fmt.Errorf("error creating request: %s", err)
	}

	openRegistryReq.Header.Set("Authorization", utils.CliConfig.Server.ApiKey)

	openRegistryResp, err := http.DefaultClient.Do(openRegistryReq)
	if err != nil {
		return fmt.Errorf("error sending request: %s", err)
	}
	defer openRegistryResp.Body.Close()

	body, err := io.ReadAll(openRegistryResp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %s", err)
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
			return fmt.Errorf("error pushing image: %w", err)
		}
		defer pushResp.Close()

		if _, err := io.Copy(os.Stdout, pushResp); err != nil {
			return fmt.Errorf("error streaming push output: %w", err)
		}
	}

	req, err := http.NewRequest("POST", utils.CliConfig.Server.Url+"/api/apps", &requestBody)
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

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %s", err)
	}

	var bodyJson types.ApiError
	json.Unmarshal(body, &bodyJson)

	if bodyJson.Code == constants.ErrorAppInstalled && !update {
		return uploadApp(appFilePath, true)
	}

	closeRegistryReq, err := http.NewRequest("DELETE", utils.CliConfig.Server.Url+"/api/registry", nil)
	if err != nil {
		return fmt.Errorf("error creating request: %s", err)
	}

	closeRegistryReq.Header.Set("Authorization", utils.CliConfig.Server.ApiKey)

	_, err = http.DefaultClient.Do(closeRegistryReq)
	if err != nil {
		return fmt.Errorf("error sending request: %s", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		fmt.Printf("[Error -> %s] %s\n\n", bodyJson.Code, bodyJson.Message)
		fmt.Println(bodyJson.Error.Error())
	} else {
		fmt.Println("\nApp deployed. Find it on the server:", utils.CliConfig.Server.Url)
	}

	return nil
}
