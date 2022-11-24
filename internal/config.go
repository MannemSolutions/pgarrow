package internal

import (
	"flag"
	"fmt"
	"github.com/mannemsolutions/pgarrrow/pkg/kafka"
	"github.com/mannemsolutions/pgarrrow/pkg/pg"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

type Config struct {
	Debug       bool         `yaml:"debug"`
	LogDest     string       `yaml:"log_dest"`
	KafkaConfig kafka.Config `yaml:"kafka_config"`
	PgConfig    pg.Config    `yaml:"pg_config"`
}

const (
	envConfName     = "PGARROW_CONFIG"
	defaultConfFile = "/etc/pgarrow/config.yaml"
)

var (
	debug      bool
	version    bool
	configFile string
)

func ProcessFlags() (err error) {
	if configFile != "" {
		return
	}

	flag.BoolVar(&debug, "d", false, "Add debugging output")
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
	config.Initialize()
	if debug {
		config.Debug = true
	}

	return config, err
}

func (config *Config) Initialize() {
	config.KafkaConfig.Initialize()
	config.PgConfig.Initialize()
	config.PgConfig.DSN["replication"] = "database"
}
