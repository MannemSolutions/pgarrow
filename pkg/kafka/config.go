package kafka

import (
	"fmt"
	"github.com/segmentio/kafka-go"
	"time"
)

type Config struct {
	Deadline      time.Duration `yaml:"deadline"`
	MessageBytes  int           `yaml:"max_message_bytes"`
	MaxBatchBytes int           `yaml:"max_batch_bytes"`
	MinBatchBytes int           `yaml:"min_batch_bytes"`
	Network       string        `yaml:"network"`
	Brokers       []string      `yaml:"brokers"`
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
		// Minimum record size would be 96 bytes at the absolute bare minimum
		c.MessageBytes = 96
	}
	if c.MinBatchBytes < 1 {
		// 1MB (maybe derive sane defaults for performance tests?)
		c.MinBatchBytes = 1024
	}
	if c.MaxBatchBytes < 1 {
		// 1MB (maybe derive sane defaults for performance tests?)
		c.MaxBatchBytes = 1048576
	}
	if c.Prefix == "" {
		c.Prefix = "pgarrow"
	}
	if c.Network == "" {
		c.Network = "tcp"
	}
	if len(c.Brokers) == 0 {
		c.Brokers = []string{"localhost:9092"}
	}
	if c.topics == nil {
		c.topics = make(Topics)
	}
	return nil
}

func (c *Config) ReaderConfig(topicName string) (r kafka.ReaderConfig) {
	topicName = fmt.Sprintf("%s_%s", c.Prefix, topicName)
	return kafka.ReaderConfig{
		Brokers:   c.Brokers,
		Topic:     topicName,
		Partition: c.Partition,
		MinBytes:  c.MinBatchBytes,
		MaxBytes:  c.MaxBatchBytes,
	}
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
