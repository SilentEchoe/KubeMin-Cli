package config

import "KubeMin-Cli/pkg/apiserver/infrastructure/datastore"

type Config struct {
	BinAddr string
	// Datastore config
	Datastore datastore.Config
}

func NewConfig() *Config {
	return &Config{
		BinAddr: "0.0.0.0:8000",
		Datastore: datastore.Config{
			Type:     "kubeapi",
			Database: "kubemincli",
			URL:      "Data Source=127.0.0.1;Database=kubemin;User Id=root;Password=123456;",
		},
	}
}
