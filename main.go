package main

import (
	"localapps-cli/cmd"
	"localapps-cli/utils"
)

func main() {
	utils.UpdateCliConfigCache()
	cmd.Execute()
}
