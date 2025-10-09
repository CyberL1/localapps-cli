package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"localapps-cli/constants"
	"localapps-cli/resources"
	"localapps-cli/types"
	"localapps-cli/utils"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func init() {
	rootCmd.AddCommand(devCmd)
}

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Run your app locally in an emulated environment",
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

		cli, _ := client.NewClientWithOpts(client.FromEnv)

		_, err = cli.Ping(context.Background())
		if err != nil {
			fmt.Println("Failed to connect to Docker daemon. Is it running?")
			return
		}

		var appId string
		if app.Id != "" {
			appId = app.Id
		} else {
			appId = strings.ToLower(strings.ReplaceAll(app.Name, " ", "-"))
		}

		for partName, part := range app.Parts {
			dockerfileVariant := "Dockerfile.dev"

			if _, err := os.Stat(part.Src + "/Dockerfile.dev"); os.IsNotExist(err) {
				println("⚠ Part", partName, "has no Dockerfile.dev file, using Dockerfile instead")
				dockerfileVariant = "Dockerfile"
			}

			fmt.Println("Building " + partName)

			buildCmd := exec.Command("docker", "build", "-t", "localapps/apps/"+appId+"/"+partName, part.Src, "-f", part.Src+"/"+dockerfileVariant)

			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr

			buildCmd.Run()
		}

		config := container.Config{
			Image:        "amir20/dozzle",
			ExposedPorts: nat.PortSet{"8080": struct{}{}},
			Env: []string{
				"DOZZLE_ENABLE_ACTIONS=true",
				"DOZZLE_ENABLE_SHELL=true",
				"DOZZLE_FILTER=\"name=localapps-app-\"",
			},
		}

		hostConfig := container.HostConfig{
			AutoRemove:   true,
			Binds:        []string{"/var/run/docker.sock:/var/run/docker.sock"},
			PortBindings: nat.PortMap{"8080": {{HostIP: "127.0.0.1", HostPort: "8081"}}},
		}

		dozzleContainer, _ := cli.ContainerCreate(context.Background(), &config, &hostConfig, nil, nil, "localapps-dev-dozzle")
		cli.ContainerStart(context.Background(), dozzleContainer.ID, container.StartOptions{})

		if err := cli.ContainerStart(context.Background(), dozzleContainer.ID, container.StartOptions{}); err != nil {
			fmt.Printf("Failed to start dozzle logger: %s\n", err)
			return
		}

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			var dockerAppName string
			var dockerImageName string

			var currentPartSource string
			var fallbackPartSource string

			var fallbackPartName string

			for name, part := range app.Parts {
				if part.Path == "" {
					fallbackPartName = name
					fallbackPartSource = part.Src
				}

				if strings.Split(r.URL.Path, "/")[1] == part.Path {
					dockerAppName = "localapps-app-" + appId + "-" + name
					dockerImageName = "localapps/apps/" + appId + "/" + name
					currentPartSource = part.Src
					break
				} else {
					dockerAppName = "localapps-app-" + appId + "-" + fallbackPartName
					dockerImageName = "localapps/apps/" + appId + "/" + fallbackPartName
					currentPartSource = fallbackPartSource
				}
			}

			containersByName, _ := cli.ContainerList(context.Background(), container.ListOptions{
				Filters: filters.NewArgs(
					filters.Arg("name", dockerAppName),
				),
			})

			var freePort int
			if len(containersByName) > 0 {
				appContainer := containersByName[0]

				containerPort := appContainer.Ports[0].PublicPort
				freePort = int(containerPort)
			} else {
				freePort, _ = utils.GetFreePort()

				config := container.Config{
					Image: dockerImageName,
					Env:   []string{"PORT=80"},
					ExposedPorts: nat.PortSet{
						"80": struct{}{},
					},
					Labels: map[string]string{
						"dev.dozzle.name": strings.Split(dockerImageName, "/")[3],
					},
				}

				hostConfig := container.HostConfig{
					Mounts: []mount.Mount{{Type: mount.TypeVolume, Source: "localapps-storage-" + appId, Target: "/storage"},
						{Type: mount.TypeBind, Source: filepath.Join(currentDir, currentPartSource), Target: "/app"}},
					PortBindings: nat.PortMap{
						"80": {
							{
								HostIP:   "0.0.0.0",
								HostPort: strconv.Itoa(freePort),
							},
						},
					},
				}

				createdContainer, _ := cli.ContainerCreate(context.Background(), &config, &hostConfig, nil, nil, dockerAppName)

				if err := cli.ContainerStart(context.Background(), createdContainer.ID, container.StartOptions{}); err != nil {
					w.Write([]byte(fmt.Sprintf("Failed to start app \"%s\": %s", appId, err)))
					return
				}
			}

			// Wait for the app to be ready
			ready := make(chan bool)
			go func() {
				for {
					resp, err := http.Get(fmt.Sprintf("http://localhost:%d", freePort))
					if err == nil && resp.StatusCode < 500 {
						ready <- true
						return
					}
					time.Sleep(300 * time.Millisecond)
				}
			}()

			select {
			case <-ready:
				appUrl, _ := url.Parse(fmt.Sprintf("http://localhost:%d", freePort))
				httputil.NewSingleHostReverseProxy(appUrl).ServeHTTP(w, r)
			case <-time.After(20 * time.Second):
				logReader, _ := cli.ContainerLogs(context.Background(), dockerAppName, container.LogsOptions{
					ShowStdout: true,
					ShowStderr: true,
					Timestamps: false,
				})
				defer logReader.Close()

				var combinedBuf bytes.Buffer
				multiWriter := io.MultiWriter(&combinedBuf)
				stdcopy.StdCopy(multiWriter, multiWriter, logReader)

				logs := combinedBuf.String()

				w.WriteHeader(http.StatusInternalServerError)
				w.Header().Set("Content-Type", "text/html")

				fileContent, err := resources.Resources.ReadFile(filepath.Join("pages", "error.html"))
				if err != nil {
					http.Error(w, fmt.Sprintf("Error reading file: %s", err), http.StatusInternalServerError)
					return
				}

				templ, _ := template.New("error.html").Parse(string(fileContent))
				template.Must(templ.Clone()).Execute(w, struct {
					ErrorCode string
					Message   string
					Logs      string
				}{
					ErrorCode: constants.ErrorContainerTimeout,
					Message:   "The app timed out",
					Logs:      string(logs),
				})
			}
		})

		// Exit handler
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-stop

			cmd.Println("Stopping development containers")
			containers, err := cli.ContainerList(context.Background(), container.ListOptions{
				Filters: filters.NewArgs(filters.Arg("name", "localapps-")),
				All:     true,
			})
			if err != nil {
				cmd.Printf("Failed to list containers: %s\n", err)
				return
			}

			for _, c := range containers {
				if err := cli.ContainerRemove(context.Background(), c.ID, container.RemoveOptions{Force: true}); err != nil {
					cmd.Printf("Failed to stop development container: %s\n", err)
					break
				}
			}
			os.Exit(0)
		}()

		cmd.Println("Your app is ready on http://localhost:8080")
		cmd.Println("App's logs are accessible on http://localhost:8081")

		if err := http.ListenAndServe(":8080", nil); err != nil {
			fmt.Printf("Failed to bind to port 8080: %s\n", err)
			os.Exit(1)
		}
	},
}
