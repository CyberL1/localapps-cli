package utils

import (
	"encoding/json"
	"fmt"
	"localapps-cli/constants"
	"os"
	"path/filepath"

	"localapps-cli/types"
)

var CliConfig types.CliConfig

func UpdateCliConfigCache() error {
	err := validateCliConfig()
	if err != nil {
		return err
	}

	configFile, err := os.Open(filepath.Join(constants.LocalappsDir, "cli-config.json"))
	if err != nil {
		return fmt.Errorf("cannot find cli config file: %s", err)
	}

	defer configFile.Close()
	decoder := json.NewDecoder(configFile)
	var config types.CliConfig
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("failed to decode cli config: %s", err)
	}

	CliConfig = config
	return nil
}

func validateCliConfig() error {
	_, err := os.Open(filepath.Join(constants.LocalappsDir, "cli-config.json"))
	if err != nil {
		os.WriteFile(filepath.Join(constants.LocalappsDir, "cli-config.json"), []byte("{\"server\":{\"url\":\"http://localhost:8080\"}}"), 0644)
	}
	return nil
}

func SaveCliConfig() error {
	data, err := json.Marshal(CliConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal CliConfig: %s", err)
	}
	os.WriteFile(filepath.Join(constants.LocalappsDir, "cli-config.json"), data, 0644)
	return nil
}
