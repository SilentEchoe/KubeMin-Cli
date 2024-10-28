package config

import (
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"fmt"
	"github.com/spf13/pflag"
)

type Config struct {
	// api server bind address
	BindAddr string
	//DTM Distributed transaction management
	DTMAddr string
	// Datastore config
	Datastore datastore.Config
	// LocalCluster
	LocalCluster bool
	// Istio Enable
	IstioEnable bool
}

func NewConfig() *Config {
	return &Config{
		BindAddr: "0.0.0.0:8000",
		Datastore: datastore.Config{
			Type:     "kubeapi",
			Database: "kubemincli",
			URL:      "Data Source=127.0.0.1;Database=kubemin;User Id=root;Password=123456;",
		},
		LocalCluster: true,
		IstioEnable:  false,
		DTMAddr:      "",
	}
}

func (c *Config) Validate() []error {
	var errs []error
	//Currently, only redis is supported
	if c.Datastore.Type != REDIS {
		errs = append(errs, fmt.Errorf("not support datastore type %s", c.Datastore.Type))
	}
	return errs
}

// AddFlags adds flags to the specified FlagSet
func (c *Config) AddFlags(fs *pflag.FlagSet, configParameter *Config) {
	fs.StringVar(&c.BindAddr, "bind-addr", configParameter.BindAddr, "The bind address used to serve the http APIs.")
	fs.StringVar(&c.Datastore.Type, "datastore-type", configParameter.Datastore.Type, "Metadata storage driver type, support mysql")
	fs.StringVar(&c.Datastore.Database, "datastore-database", configParameter.Datastore.Database, "Metadata storage database name, takes effect when the storage driver is mysql.")
	fs.StringVar(&c.Datastore.URL, "datastore-url", configParameter.Datastore.URL, "Metadata storage database url,takes effect when the storage driver is mysql.")
	AddFlags(fs)
}

// AddFlags .
func AddFlags(fs *pflag.FlagSet) {
	var Addr = ""
	fs.StringVarP(&Addr, "profiling-addr", "", Addr, "if not empty, start the profiling server at the given address")
}
