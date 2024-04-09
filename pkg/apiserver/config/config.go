package config

import (
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"fmt"
)

const (
	Mysql = "mysql"
)

type Config struct {
	// api server bind address
	BinAddr string
	// Datastore config
	Datastore     datastore.Config
	DatastoreType string
	// LocalCluster
	LocalCluster bool
	// Istio Enable
	IstioEnable bool
}

func NewConfig() *Config {
	return &Config{
		BinAddr: "0.0.0.0:8000",
		Datastore: datastore.Config{
			Type:     "kubeapi",
			Database: "kubemincli",
			URL:      "Data Source=127.0.0.1;Database=kubemin;User Id=root;Password=123456;",
		},
		LocalCluster:  true,
		DatastoreType: Mysql,
		IstioEnable:   false,
	}
}

func (c *Config) Validate() []error {
	var errs []error

	if c.DatastoreType != "mysql" {
		errs = append(errs, fmt.Errorf("not support datastore type %s", c.DatastoreType))
	}
	return errs
}
