package kafka

import (
	"fmt"
	"time"
)

type Config struct {
	Deadline      time.Duration `yaml:"deadline"`
	MessageBytes  int           `yaml:"max_message_bytes"`
	MaxBatchBytes int           `yaml:"max_batch_bytes"`
	MinBatchBytes int           `yaml:"min_batch_bytes"`
	Network       string        `yaml:"network"`
	Address       string        `yaml:"address"`
	Prefix        string        `yaml:"topic_prefix"`
	Partition     int           `yaml:"partition"`
	topics        Topics
}

// Initialize will initialize the config with defaults
func (c *Config) Initialize() (err error) {
	if c.Deadline.Milliseconds() < 1 {
		c.Deadline = time.Second * 10
	}
	if c.MessageBytes < 1 {
		// 4k (maybe derive sane defaults for performance tests?)
		c.MessageBytes = 4096
	}
	if c.MaxBatchBytes < 1 {
		// 1MB (maybe derive sane defaults for performance tests?)
		c.MaxBatchBytes = 1048576
	}
	if c.Prefix == "" {
		c.Prefix = "pgarrow"
	}
	if c.topics == nil {
		fmt.Println("make topics")
		c.topics = make(Topics)
	}
	return nil
}

func (c *Config) Close() (err error) {
	for _, t := range c.topics {
		if err = t.Close(); err != nil {
			return err
		}
		delete(c.topics, t.name)
	}
	return nil
}

func (c *Config) NewTopic(name string) *Topic {
	name = fmt.Sprintf("%s_%s", c.Prefix, name)
	if t, exists := c.topics[name]; exists {
		return t
	}

	t := Topic{
		name:   name,
		parent: c,
	}
	if c.topics == nil {
		fmt.Println("not make topics")
	}
	c.topics[name] = &t

	return &t
}
