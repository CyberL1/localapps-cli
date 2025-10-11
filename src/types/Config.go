package types

type CliConfig struct {
	Server CliConfigServer `json:"server"`
}

type CliConfigServer struct {
	Url    string `json:"url"`
	ApiKey string `json:"apiKey"`
}
