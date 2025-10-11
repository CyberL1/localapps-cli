package main

import (
	"fmt"
	"localapps-cli/cmd"
	"localapps-cli/constants"
	"localapps-cli/utils"
	"os"
)

func main() {
	// Check for all required resources
	if _, err := os.Stat(constants.LocalappsDir); os.IsNotExist(err) {
		if err := os.Mkdir(constants.LocalappsDir, 0755); err != nil {
			fmt.Printf("Failed to create %s directory: %s\n", constants.LocalappsDir, err)
			return
		}
	}

	utils.UpdateCliConfigCache()
	cmd.Execute()
}
