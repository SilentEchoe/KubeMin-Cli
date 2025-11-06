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
	// CacheTTL sets default ttl for ICache entries
	CacheTTL time.Duration
	// KeyPrefix applied to cache keys in redis
	KeyPrefix string
}

// MessagingConfig holds pub/sub configuration
type MessagingConfig struct {
	Type          string // noop|redis|kafka
	ChannelPrefix string
	// RedisStreamMaxLen sets XADD MAXLEN to cap stream length (<=0 disables).
	RedisStreamMaxLen int64
}

func NewConfig() *Config {
	return &Config{
		BindAddr: "0.0.0.0:8000",
		LeaderConfig: leaderConfig{
			ID:       uuid.New().String(),
			LockName: "apiserver-lock",
			//Duration:  time.Second * 5,
			Duration:  time.Minute * 60,
			Namespace: NAMESPACE,
		},
		Datastore: datastore.Config{
			Type:            MYSQL,
			Database:        DBNAME_KUBEMINCLI,
			URL:             fmt.Sprintf("root:123456@tcp(127.0.0.1:3306)/%s?charset=utf8&parseTime=true", DBNAME_KUBEMINCLI),
			MaxIdleConns:    10,
			MaxOpenConns:    100,
			ConnMaxLifetime: 30 * time.Minute,
			ConnMaxIdleTime: 10 * time.Minute,
		},
		Cache: RedisCacheConfig{
			CacheHost: "localhost",
			CacheProt: 6379,
			CacheType: "redis",
			UserName:  "",
			Password:  "",
			CacheDB:   0,
			CacheTTL:  24 * time.Hour,
			KeyPrefix: "kubemin:cache:",
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
		Messaging: MessagingConfig{Type: "redis", RedisStreamMaxLen: 50000},
	}
}

func (c *Config) Validate() []error {
	var errs []error
	if strings.TrimSpace(c.BindAddr) == "" {
		errs = append(errs, fmt.Errorf("bind address cannot be empty"))
	}
	if c.Datastore.Type == MYSQL && strings.TrimSpace(c.Datastore.URL) == "" {
		errs = append(errs, fmt.Errorf("mysql url cannot be empty"))
	}
	if c.Cache.CacheType == ("redis") {
		if strings.TrimSpace(c.Cache.CacheHost) == "" || c.Cache.CacheProt <= 0 {
			errs = append(errs, fmt.Errorf("redis cache host/port is invalid"))
		}
	}
	// messaging basic checks
	switch strings.ToLower(strings.TrimSpace(c.Messaging.Type)) {
	case "", "noop", "redis", "kafka":
		// ok
	default:
		errs = append(errs, fmt.Errorf("unsupported messaging type: %s", c.Messaging.Type))
	}
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
	fs.StringVar(&c.Datastore.Type, "datastore-type", configParameter.Datastore.Type, "datastore backend type (e.g., mysql, tidb)")
	fs.StringVar(&c.Datastore.URL, "datastore-url", configParameter.Datastore.URL, "datastore connection URL / DSN")
	fs.StringVar(&c.Datastore.Database, "datastore-database", configParameter.Datastore.Database, "datastore database/schema name")
	fs.IntVar(&c.Datastore.MaxIdleConns, "mysql-max-idle-conns", configParameter.Datastore.MaxIdleConns, "maximum number of idle MySQL connections to retain in the pool")
	fs.IntVar(&c.Datastore.MaxOpenConns, "mysql-max-open-conns", configParameter.Datastore.MaxOpenConns, "maximum number of open MySQL connections (<=0 means unlimited)")
	fs.DurationVar(&c.Datastore.ConnMaxLifetime, "mysql-conn-max-lifetime", configParameter.Datastore.ConnMaxLifetime, "maximum amount of time a MySQL connection may be reused (<=0 disables)")
	fs.DurationVar(&c.Datastore.ConnMaxIdleTime, "mysql-conn-max-idle-time", configParameter.Datastore.ConnMaxIdleTime, "maximum amount of time a MySQL connection may remain idle (<=0 disables)")
	fs.BoolVar(&c.EnableTracing, "enable-tracing", configParameter.EnableTracing, "Enable distributed tracing.")
	fs.BoolVar(&c.AutoTracing, "auto-tracing", configParameter.AutoTracing, "Auto-enable tracing when Jaeger is configured or messaging is redis (effective only if --enable-tracing=false).")
	fs.StringVar(&c.JaegerEndpoint, "jaeger-endpoint", configParameter.JaegerEndpoint, "The endpoint of the Jaeger collector.")
	// messaging basic flags (broker type & channel prefix). Redis connection will reuse RedisCacheConfig.
	fs.StringVar(&c.Messaging.Type, "msg-type", configParameter.Messaging.Type, "messaging broker type: noop|redis|kafka")
	fs.StringVar(&c.Messaging.ChannelPrefix, "msg-channel-prefix", configParameter.Messaging.ChannelPrefix, "messaging channel prefix for topics")
	fs.Int64Var(&c.Messaging.RedisStreamMaxLen, "msg-redis-maxlen", configParameter.Messaging.RedisStreamMaxLen, "redis streams XADD MAXLEN cap (<=0 to disable)")
	// cache-specific flags
	fs.StringVar(&c.Cache.CacheType, "cache-type", configParameter.Cache.CacheType, "cache backend type (redis|memory)")
	fs.StringVar(&c.Cache.CacheHost, "cache-host", configParameter.Cache.CacheHost, "cache host for redis backend")
	fs.IntVar(&c.Cache.CacheProt, "cache-port", configParameter.Cache.CacheProt, "cache port for redis backend")
	fs.Int64Var(&c.Cache.CacheDB, "cache-db", configParameter.Cache.CacheDB, "cache database index for redis backend")
	fs.StringVar(&c.Cache.UserName, "cache-username", configParameter.Cache.UserName, "cache username for redis backend")
	fs.StringVar(&c.Cache.Password, "cache-password", configParameter.Cache.Password, "cache password for redis backend")
	fs.DurationVar(&c.Cache.CacheTTL, "cache-ttl", configParameter.Cache.CacheTTL, "default TTL for redis cache entries (e.g. 24h)")
	fs.StringVar(&c.Cache.KeyPrefix, "cache-prefix", configParameter.Cache.KeyPrefix, "key prefix for redis cache entries")
	// profiling flags live in the profiling package; wire them here for convenience
	profiling.AddFlags(fs)
}

// HasExternalQueue returns true if a non-noop messaging backend is configured,
// which typically implies a distributed queue (e.g., redis, kafka, nsq).
func (c Config) HasExternalQueue() bool {
	t := strings.ToLower(strings.TrimSpace(c.Messaging.Type))
	return t != "" && t != "noop"
}
