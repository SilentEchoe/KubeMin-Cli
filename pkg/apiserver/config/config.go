package config

import (
	"fmt"
	"github.com/spf13/pflag"
)

type Config struct {
	// api server bind address
	BindAddr string
	//DTM Distributed transaction management
	DTMAddr string
	// LocalCluster
	LocalCluster bool
	// KubeConfig
	KubeConfig string
	// Istio Enable
	IstioEnable bool
}

func NewConfig() *Config {
	return &Config{
		BindAddr:     "0.0.0.0:8000",
		LocalCluster: true,
		KubeConfig:   "",
		IstioEnable:  false,
		DTMAddr:      "",
	}
}

func (c *Config) Validate() []error {
	var errs []error
	if !c.LocalCluster && c.KubeConfig == "" {
		errs = append(errs, fmt.Errorf("when localCluster is set to false, KubeConfig must be set"))
	}
	return errs
}

// AddFlags adds flags to the specified FlagSet
func (c *Config) AddFlags(fs *pflag.FlagSet, configParameter *Config) {
	fs.StringVar(&c.BindAddr, "bind-addr", configParameter.BindAddr, "The bind address used to serve the http APIs.")
	AddFlags(fs)
}

// AddFlags .
func AddFlags(fs *pflag.FlagSet) {
	var Addr = ""
	fs.StringVarP(&Addr, "profiling-addr", "", Addr, "if not empty, start the profiling server at the given address")
}
