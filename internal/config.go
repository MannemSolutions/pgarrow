package internal

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mannemsolutions/pgarrrow/pkg/kafka"
	"github.com/mannemsolutions/pgarrrow/pkg/pg"
	"github.com/mannemsolutions/pgarrrow/pkg/rabbitmq"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Direction      string          `yaml:"direction"`
	Debug          bool            `yaml:"debug"`
	LogDest        string          `yaml:"log_dest"`
	KafkaConfig    kafka.Config    `yaml:"kafka_config"`
	PgConfig       pg.Config       `yaml:"pg_config"`
	RabbitMqConfig rabbitmq.Config `yaml:"rabbit_config"`
}

const (
	envConfName      = "PGARROW_CONFIG"
	defaultConfFile  = "/etc/pgarrow/config.yaml"
	envDirectionName = "PGARROW_DIRECTION"
)

var (
	direction  string
	debug      bool
	version    bool
	configFile string
)

func ProcessFlags() (err error) {
	if configFile != "" {
		return
	}

	flag.StringVar(&direction, "d", os.Getenv(envDirectionName), "Direction, options are: pgarrowkafka kafkaarrowpg "+
		"pgarrowrabbit rabbitarrowpg")
	flag.BoolVar(&debug, "x", false, "Add debugging output")
	flag.BoolVar(&version, "v", false, "Show version information")

	flag.StringVar(&configFile, "c", os.Getenv(envConfName), "Path to configfile")

	flag.Parse()

	if version {
		//nolint
		fmt.Println(AppVersion)
		os.Exit(0)
	}

	if configFile == "" {
		configFile = defaultConfFile
	}

	configFile, err = filepath.EvalSymlinks(configFile)
	return err
}

func NewConfig() (config Config, err error) {
	if err = ProcessFlags(); err != nil {
		return
	}

	// This only parsed as yaml, nothing else
	// #nosec
	yamlConfig, err := os.ReadFile(configFile)
	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(yamlConfig, &config)
	if debug {
		config.Debug = true
	}

	config.Direction = direction
	config.Initialize()

	return config, err
}

func (config *Config) Initialize() {
	config.RabbitMqConfig.Initialize()
	config.KafkaConfig.Initialize()
	config.PgConfig.Initialize()
	config.PgConfig.DSN["replication"] = "database"
}
