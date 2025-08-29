package config

import (
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/pflag"
)

var (
	// Addr the address for starting profiling server
	Addr = ""
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

	Datastore datastore.Config

	Cache RedisCacheConfig

	// Istio Enable
	IstioEnable bool

	// EnableTracing enables distributed tracing
	EnableTracing bool

	// JaegerEndpoint is the endpoint of the Jaeger collector
	JaegerEndpoint string

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

type RedisCacheConfig struct {
	CacheHost string
	CacheType string
	CacheDB   int64
	UserName  string
	Password  string
}

func NewConfig() *Config {
	return &Config{
		BindAddr: "0.0.0.0:8000",
		LeaderConfig: leaderConfig{
			ID:       uuid.New().String(),
			LockName: "apiserver-lock",
			Duration: time.Second * 5,
		},
		Datastore: datastore.Config{
			Type:     MYSQL,
			Database: DBNAME_KUBEMINCLI,
			URL:      fmt.Sprintf("root:123456@tcp(127.0.0.1:3306)/%s?charset=utf8&parseTime=true", DBNAME_KUBEMINCLI),
		},
		KubeQPS:          100,
		KubeBurst:        300,
		AddonCacheTime:   time.Minute * 10,
		IstioEnable:      false,
		ExitOnLostLeader: true,
		DTMAddr:          "",
		EnableTracing:    true,
		JaegerEndpoint:   "",
		//JaegerEndpoint:   "http://localhost:14268/api/traces",
	}
}

func (c *Config) Validate() []error {
	var errs []error
	return errs
}

// AddFlags adds flags to the specified FlagSet
func (c *Config) AddFlags(fs *pflag.FlagSet, configParameter *Config) {
	fs.StringVar(&c.BindAddr, "bind-addr", configParameter.BindAddr, "The bind address used to serve the http APIs.")
	fs.StringVar(&c.LeaderConfig.ID, "id", configParameter.LeaderConfig.ID, "the holder identity name")
	fs.StringVar(&c.LeaderConfig.LockName, "lock-name", configParameter.LeaderConfig.LockName, "the lease lock resource name")
	fs.DurationVar(&c.LeaderConfig.Duration, "duration", configParameter.LeaderConfig.Duration, "the lease lock resource name")
	fs.Float64Var(&c.KubeQPS, "kube-api-qps", configParameter.KubeQPS, "the qps for kube clients. Low qps may lead to low throughput. High qps may give stress to api-server.")
	fs.IntVar(&c.KubeBurst, "kube-api-burst", configParameter.KubeBurst, "the burst for kube clients. Recommend setting it qps*3.")
	fs.BoolVar(&c.ExitOnLostLeader, "exit-on-lost-leader", configParameter.ExitOnLostLeader, "exit the process if this server lost the leader election")
	fs.BoolVar(&c.EnableTracing, "enable-tracing", configParameter.EnableTracing, "Enable distributed tracing.")
	fs.StringVar(&c.JaegerEndpoint, "jaeger-endpoint", configParameter.JaegerEndpoint, "The endpoint of the Jaeger collector.")
	addFlags(fs)
}

func addFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&Addr, "profiling-addr", "", Addr, "if not empty, start the profiling server at the given address")
}
