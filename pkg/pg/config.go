package pg

import "time"

type Config struct {
	DSN                   Dsn           `yaml:"dsn"`
	Slot                  string        `yaml:"slot_name"`
	standbyMessageTimeout time.Duration `yaml:"standby_message_timeout"`
}

// Initialize currently has no function, but can be used to initialize teh config with defaults
func (c *Config) Initialize() (err error) {
	if c.Slot == "" {
		c.Slot = "pgarrow"
	}
	if c.standbyMessageTimeout.Milliseconds() < 1 {
		c.standbyMessageTimeout = time.Second * 10
	}
	return nil
}
