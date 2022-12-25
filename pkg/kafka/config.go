package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type Config struct {
	Brokers       []string      `yaml:"brokers"`
	Deadline      time.Duration `yaml:"deadline"`
	MaxBatchBytes int           `yaml:"max_batch_bytes"`
	MinBatchBytes int           `yaml:"min_batch_bytes"`
	Prefix        string        `yaml:"topic_prefix"`
	ConsumerGroup string        `yaml:"consumer_group"`
	topics        Topics
}

// Initialize will initialize the config with defaults
func (c *Config) Initialize() (err error) {
	if c.Deadline.Milliseconds() < 1 {
		c.Deadline = time.Second
	}
	if c.MinBatchBytes < 1 {
		// 1MB (maybe derive sane defaults for performance tests?)
		c.MinBatchBytes = 96
	}
	if c.MaxBatchBytes < 1 {
		// 1MB (maybe derive sane defaults for performance tests?)
		c.MaxBatchBytes = 1048576
	}
	if c.ConsumerGroup == "" {
		c.ConsumerGroup = "pgarrow1"
	}
	if c.Prefix == "" {
		c.Prefix = "pgarrow"
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
		Brokers:  c.Brokers,
		Topic:    topicName,
		GroupID:  c.ConsumerGroup,
		MinBytes: c.MinBatchBytes,
		MaxBytes: c.MaxBatchBytes,
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
	c.topics[name] = &t

	return &t
}

func (c Config) Context() (context.Context, context.CancelFunc) {
	return context.WithDeadline(ctx, time.Now().Add(c.Deadline))
}
