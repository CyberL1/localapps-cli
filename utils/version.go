package utils

import (
	"encoding/json"
	"io"
	"localapps-cli/constants"
	"localapps-cli/types"
	"net/http"
)

func GetLatestCliVersion() (*types.GithubRelease, error) {
	resp, err := http.Get(constants.GithubReleaseUrl)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	release := &types.GithubRelease{}
	err = json.Unmarshal(body, release)
	if err != nil {
		return nil, err
	}
	return release, nil
}
