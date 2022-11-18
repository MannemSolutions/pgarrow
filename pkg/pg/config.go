package pg

type Config struct {
	DSN                   Dsn    `yaml:"dsn"`
	Slot                  string `yaml:"slot_name"`
	standbyMessageTimeout uint   `yaml:"standby_message_timeout"`
}

// Initialize currently has no function, but can be used to initialize teh config with defaults
func (c *Config) Initialize() (err error) {
	return nil
}
