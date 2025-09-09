package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/pflag"

	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/utils/profiling"
)

type leaderConfig struct {
	ID        string
	LockName  string
	Duration  time.Duration
	Namespace string
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

	// AutoTracing, when true and EnableTracing is false, auto-enables tracing
	// if a supported exporter is configured or a distributed queue is used.
	AutoTracing bool

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
	// Messaging configuration (pub/sub)
	Messaging MessagingConfig
}

type RedisCacheConfig struct {
	CacheHost string
	CacheProt int
	CacheType string
	CacheDB   int64
	UserName  string
	Password  string
}

// MessagingConfig holds pub/sub configuration
type MessagingConfig struct {
	Type          string // noop|redis|kafka
	ChannelPrefix string
}

func NewConfig() *Config {
	return &Config{
		BindAddr: "0.0.0.0:8000",
		LeaderConfig: leaderConfig{
			ID:        uuid.New().String(),
			LockName:  "apiserver-lock",
			Duration:  time.Second * 5,
			Namespace: NAMESPACE,
		},
		Datastore: datastore.Config{
			Type:     MYSQL,
			Database: DBNAME_KUBEMINCLI,
			URL:      fmt.Sprintf("root:123456@tcp(127.0.0.1:3306)/%s?charset=utf8&parseTime=true", DBNAME_KUBEMINCLI),
		},
		Cache: RedisCacheConfig{
			CacheHost: "localhost",
			CacheProt: 6379,
			CacheType: "redis",
			UserName:  "",
			Password:  "",
			CacheDB:   0,
		},
		KubeQPS:          100,
		KubeBurst:        300,
		AddonCacheTime:   time.Minute * 10,
		IstioEnable:      false,
		ExitOnLostLeader: true,
		DTMAddr:          "",
		EnableTracing:    true,
		AutoTracing:      false,
		JaegerEndpoint:   "",
		//JaegerEndpoint:   "http://localhost:14268/api/traces",
		Messaging: MessagingConfig{Type: "redis"},
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
	fs.DurationVar(&c.LeaderConfig.Duration, "duration", configParameter.LeaderConfig.Duration, "leader election lease duration (e.g.15s)")
	fs.StringVar(&c.LeaderConfig.Namespace, "leader-namespace", configParameter.LeaderConfig.Namespace, "namespace for leader election lease")
	fs.Float64Var(&c.KubeQPS, "kube-api-qps", configParameter.KubeQPS, "the qps for kube clients. Low qps may lead to low throughput. High qps may give stress to api-server.")
	fs.IntVar(&c.KubeBurst, "kube-api-burst", configParameter.KubeBurst, "the burst for kube clients. Recommend setting it qps*3.")
	fs.BoolVar(&c.ExitOnLostLeader, "exit-on-lost-leader", configParameter.ExitOnLostLeader, "exit the process if this server lost the leader election")
	fs.BoolVar(&c.EnableTracing, "enable-tracing", configParameter.EnableTracing, "Enable distributed tracing.")
	fs.BoolVar(&c.AutoTracing, "auto-tracing", configParameter.AutoTracing, "Auto-enable tracing when Jaeger is configured or messaging is redis (effective only if --enable-tracing=false).")
	fs.StringVar(&c.JaegerEndpoint, "jaeger-endpoint", configParameter.JaegerEndpoint, "The endpoint of the Jaeger collector.")
	// messaging basic flags (broker type & channel prefix). Redis connection will reuse RedisCacheConfig.
	fs.StringVar(&c.Messaging.Type, "msg-type", configParameter.Messaging.Type, "messaging broker type: noop|redis|kafka")
	fs.StringVar(&c.Messaging.ChannelPrefix, "msg-channel-prefix", configParameter.Messaging.ChannelPrefix, "messaging channel prefix for topics")
	// profiling flags live in the profiling package; wire them here for convenience
	profiling.AddFlags(fs)
}

// HasExternalQueue returns true if a non-noop messaging backend is configured,
// which typically implies a distributed queue (e.g., redis, kafka, nsq).
func (c Config) HasExternalQueue() bool {
	t := strings.ToLower(strings.TrimSpace(c.Messaging.Type))
	return t != "" && t != "noop"
}
