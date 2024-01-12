package config

import "KubeMin-Cli/pkg/apiserver/infrastructure/datastore"

const (
	Mysql = "mysql"
)

type Config struct {
	BinAddr string
	// Datastore config
	Datastore     datastore.Config
	DatastoreType string
	LocalCluster  bool
}

func NewConfig() *Config {
	return &Config{
		BinAddr: "0.0.0.0:8000",
		Datastore: datastore.Config{
			Type:     "kubeapi",
			Database: "kubemincli",
			URL:      "Data Source=127.0.0.1;Database=kubemin;User Id=root;Password=123456;",
		},
		LocalCluster:  true,  //默认本地集群,
		DatastoreType: Mysql, //默认为Mysql
	}
}
