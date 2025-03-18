package config

const (
	REDIS             = "redis"
	TIDB              = "tidb"
	MYSQL             = "mysql"
	DBNAME_KUBEMINCLI = "kubemincli"
)

type JobRunPolicy string
type JobType string

const (
	DefaultRun    JobRunPolicy = ""                // default run this job
	DefaultNotRun JobRunPolicy = "default_not_run" // default not run this job
	ForceRun      JobRunPolicy = "force_run"       // force run this job
	SkipRun       JobRunPolicy = "skip"

	DefaultJobBuild  JobType = "default_build"
	DefaultJobDeploy JobType = "default_deploy"
)
