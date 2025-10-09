package types

type App struct {
	Id    string          `yaml:"id"`
	Name  string          `yaml:"name"`
	Icon  string          `yaml:"icon"`
	Parts map[string]Part `yaml:"parts"`
}

type Part struct {
	Src  string `yaml:"src"`
	Path string `yaml:"path,omitempty"`
	Dev  string `yaml:"dev"`
}
