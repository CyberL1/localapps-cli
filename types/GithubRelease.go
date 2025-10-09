package types

type GithubRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool
}

type GithubFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	DownloadUrl string `json:"download_url"`
}
