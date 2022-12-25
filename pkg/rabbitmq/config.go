package rabbitmq

import (
	"context"
	"time"
)

type Config struct {
	AutoDelete bool          `yaml:"auto_delete"`
	Deadline   time.Duration `yaml:"deadline"`
	Queue      string        `yaml:"queue"`
	Transient  bool          `yaml:"transient"`
	Url        string        `yaml:"url"`
	queues     Queues
}

// Initialize will initialize the config with defaults
func (c *Config) Initialize() (err error) {
	if c.Deadline.Milliseconds() < 1 {
		c.Deadline = time.Second
	}
	if c.Queue == "" {
		c.Queue = "stream"
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
		name:   name,
		config: c,
	}
	c.queues[name] = &q

	return &q
}

func (c Config) Context() (context.Context, context.CancelFunc) {
	return context.WithDeadline(ctx, time.Now().Add(c.Deadline))
}
