package types

type ServerConfig struct {
	AccessUrl string `default:"\"http://localhost:8080\""`
	ApiKey    string `default:"\"\""`
}

type CliConfig struct {
	Server CliConfigServer `json:"server"`
}

type CliConfigServer struct {
	Url    string `json:"url"`
	ApiKey string `json:"apiKey"`
}
