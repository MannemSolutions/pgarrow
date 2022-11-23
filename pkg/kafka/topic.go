package kafka

import (
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"time"
)

type Topics map[string]*Topic

type Topic struct {
	name   string
	reader *kafka.Reader
	writer *kafka.Writer
	parent *Config
	batch  *kafka.Batch
}

func (t *Topic) Connect() (err error) {
	if err = t.ConnectReader(); err != nil {
		return err
	}
	t.ConnectWriter()
	return nil
}

func (t *Topic) ConnectReader() (err error) {
	if t.reader != nil {
		return nil
	}
	if t.reader = kafka.NewReader(t.parent.ReaderConfig(t.name)); err != nil {
		log.Fatal("failed to dial leader:", err)
	}
	return nil
}

func (t *Topic) ConnectWriter() {
	if t.writer != nil {
		return
	}
	t.writer = &kafka.Writer{
		Addr:       kafka.TCP(t.parent.Brokers...),
		Topic:      fmt.Sprintf("%s_%s", t.parent.Prefix, t.name),
		BatchBytes: int64(t.parent.MaxBatchBytes),
		Async:      true,
	}
}

func (t *Topic) MustClose() {
	if err := t.Close(); err != nil {
		log.Fatalf("Error closing kafka connection: %e", err)
	}
}

func (t *Topic) Close() (err error) {
	if t.reader == nil {
		return nil
	}
	if err = t.reader.Close(); err != nil {
		return err
	}
	t.reader = nil
	return nil
}

func (t *Topic) Publish(data []byte) (err error) {
	return t.MultiPublish([][]byte{data})
}

func (t *Topic) MultiPublish(multiData [][]byte) (err error) {
	if len(multiData) == 0 {
		return nil
	}
	t.ConnectWriter()
	var msgs []kafka.Message
	for _, d := range multiData {
		msgs = append(msgs, kafka.Message{Value: d})
	}
	if err = t.writer.WriteMessages(t.Context(), msgs...); err != nil {
		log.Fatal("failed to write messages:", err)
	}
	return nil
}

func (t *Topic) Context() context.Context {
	c, _ := context.WithDeadline(ctx, time.Now().Add(t.parent.Deadline))
	return c
}

func (t *Topic) Consume() (m kafka.Message, err error) {
	if err = t.ConnectReader(); err != nil {
		return m, err
	}
	if m, err = t.reader.FetchMessage(t.Context()); err != nil {
		return m, err
	}
	return m, nil
}
func (t *Topic) Commit(m kafka.Message) (err error) {
	return t.reader.CommitMessages(t.Context(), m)
}
