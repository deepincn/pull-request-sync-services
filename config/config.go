package config

// Yaml struct of yaml
type Yaml struct {
	Gerrit   *string `yaml:"gerrit"`
	GerritServer *string `yaml:"gerrit_server"`
	RepoDir  *string `yaml:"repo_dir"`
	Database *struct {
		Type     *string `yaml:"type"`
		FileName *string `yaml:"filename"`
	} `yaml:"database"`
	Auth *struct {
		Github *struct {
			Token    *string `yaml:"token,omitempty"`
			User     *string `yaml:"user,omitempty"`
			Password *string `yaml:"password,omitempty"`
		} `yaml:"github"`
		Gerrit *struct {
			Token    *string `yaml:"token,omitempty"`
			User     *string `yaml:"user,omitempty"`
			Password *string `yaml:"password,omitempty"`
		} `yaml:"gerrit"`
	} `yaml:"auth"`
}
