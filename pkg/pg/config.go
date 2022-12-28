package pg

import "time"

type Config struct {
	DSN                   Dsn              `yaml:"dsn"`
	Slot                  string           `yaml:"slot_name"`
	SkipErrors            map[int64]string `yaml:"skip_errors"`
	StandbyMessageTimeout time.Duration    `yaml:"standby_message_timeout"`
}

// Initialize currently has no function, but can be used to initialize teh config with defaults
func (c *Config) Initialize() (err error) {
	if len(c.DSN) == 0 {
		c.DSN = make(Dsn)
	}
	if c.Slot == "" {
		c.Slot = "pgarrow"
	}
	if c.standbyMessageTimeout.Milliseconds() < 1 {
		c.standbyMessageTimeout = time.Second * 10
	}
	return nil
}

func (c Config) Clone() (newConfig Config) {
	newConfig = Config{
		DSN:                   c.DSN.Clone(),
		Slot:                  c.Slot,
		standbyMessageTimeout: c.standbyMessageTimeout,
	}
	if err := newConfig.Initialize(); err != nil {
		log.Fatalf("failed to initialize this config: %e", err)
	}
	return newConfig
}
