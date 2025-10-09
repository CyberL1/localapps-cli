package utils

import (
	"fmt"
	"io"
	"localapps-cli/types"
	"os"

	"github.com/go-yaml/yaml"
)

func GetAppInfo() (*types.App, error) {
	appFilePath := "app.yml"

	file, err := os.Open(appFilePath)
	if err != nil {
		return nil, fmt.Errorf("app.yml not found")
	}

	defer file.Close()
	appFileContents, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read app file: %v", err)
	}

	var app types.App
	err = yaml.Unmarshal(appFileContents, &app)
	if err != nil {
		return nil, fmt.Errorf("failed to parse app file: %v", err)
	}

	return &app, nil
}
