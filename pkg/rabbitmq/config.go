package rabbitmq

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Config struct {
	AutoDelete bool          `yaml:"auto_delete"`
	Deadline   time.Duration `yaml:"deadline"`
	Prefix     string        `yaml:"prefix"`
	Transient  bool          `yaml:"transient"`
	Url        string        `yaml:"url"`
	queues     Queues
}

// Initialize will initialize the config with defaults
func (c *Config) Initialize() (err error) {
	if c.Deadline.Milliseconds() < 1 {
		c.Deadline = time.Second
	}
	if c.Prefix == "" {
		c.Prefix = "pgarrow"
	} else if strings.HasPrefix(c.Prefix, "amq.") {
		c.Prefix = fmt.Sprintf("%s.%s", "pgarrow", strings.TrimPrefix(c.Prefix, "amq."))
	}
	if c.Url == "" {
		c.Url = "amqp://arrow:arrow@localhost:5672/"
	}
	if c.queues == nil {
		c.queues = make(Queues)
	}
	return nil
}

func (c *Config) NewQueue(name string) *Queue {
	if q, exists := c.queues[name]; exists {
		return q
	}

	q := Queue{
		name:   fmt.Sprintf("%s_%s", c.Prefix, name),
		config: c,
	}
	c.queues[name] = &q

	return &q
}

func (c Config) Context() (context.Context, context.CancelFunc) {
	return context.WithDeadline(ctx, time.Now().Add(c.Deadline))
}
