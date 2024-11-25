package config

import (
	"github.com/google/uuid"
	"github.com/spf13/pflag"
	"time"
)

type leaderConfig struct {
	ID       string
	LockName string
	Duration time.Duration
}

type Config struct {
	// api server bind address
	BindAddr string

	//DTM Distributed transaction management
	DTMAddr string

	// Istio Enable
	IstioEnable bool

	// AddonCacheTime is how long between two cache operations
	AddonCacheTime time.Duration

	// LeaderConfig for leader election
	LeaderConfig leaderConfig

	// KubeBurst the burst of kube client
	KubeBurst int

	// KubeQPS the QPS of kube client
	KubeQPS float64

	//ExitOnLostLeader will exit the process if this server lost the leader election, set this to true for debugging
	ExitOnLostLeader bool
}

func NewConfig() *Config {
	return &Config{
		BindAddr: "0.0.0.0:8000",
		LeaderConfig: leaderConfig{
			ID:       uuid.New().String(),
			LockName: "apiserver-lock",
			Duration: time.Second * 5,
		},
		KubeQPS:          100,
		KubeBurst:        300,
		AddonCacheTime:   time.Minute * 10,
		IstioEnable:      false,
		ExitOnLostLeader: true,
		DTMAddr:          "",
	}
}

func (c *Config) Validate() []error {
	var errs []error
	return errs
}

// AddFlags adds flags to the specified FlagSet
func (s *Config) AddFlags(fs *pflag.FlagSet, configParameter *Config) {
	fs.StringVar(&s.BindAddr, "bind-addr", configParameter.BindAddr, "The bind address used to serve the http APIs.")
	fs.StringVar(&s.LeaderConfig.ID, "id", configParameter.LeaderConfig.ID, "the holder identity name")
	fs.StringVar(&s.LeaderConfig.LockName, "lock-name", configParameter.LeaderConfig.LockName, "the lease lock resource name")
	fs.DurationVar(&s.LeaderConfig.Duration, "duration", configParameter.LeaderConfig.Duration, "the lease lock resource name")
	fs.Float64Var(&s.KubeQPS, "kube-api-qps", configParameter.KubeQPS, "the qps for kube clients. Low qps may lead to low throughput. High qps may give stress to api-server.")
	fs.IntVar(&s.KubeBurst, "kube-api-burst", configParameter.KubeBurst, "the burst for kube clients. Recommend setting it qps*3.")
	fs.BoolVar(&s.ExitOnLostLeader, "exit-on-lost-leader", configParameter.ExitOnLostLeader, "exit the process if this server lost the leader election")
}
