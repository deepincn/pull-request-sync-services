package config

// Yaml struct of yaml
type Yaml struct {
	Gerrit   string `yaml:"gerrit"`
	RepoDir  string `yaml:"repo_dir"`
	database struct {
		Type     string `yaml:"type"`
		FileName string `yaml:"filename"`
	}
}
