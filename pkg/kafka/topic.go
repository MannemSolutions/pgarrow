package kafka

import (
	"github.com/segmentio/kafka-go"
	"time"
)

type Topic struct {
	name   string
	conn   *kafka.Conn
	parent *Config
}

func (t *Topic) Connect() (err error) {
	if t.conn != nil {
		return nil
	}
	if t.conn, err = kafka.DialLeader(ctx, "tcp", "localhost:9092", t.name,
		t.parent.Partition); err != nil {
		log.Fatal("failed to dial leader:", err)
	}
	return nil
}

func (t *Topic) Close() (err error) {
	if err = t.conn.Close(); err != nil {
		return err
	}
	t.conn = nil
	return nil
}

func (t *Topic) ExtendDeadline(duration time.Duration) {
	t.conn.SetDeadline(time.Now().Add(duration))
}

func (t *Topic) Publish(data []byte) (err error) {
	return t.MultiPublish([][]byte{data})
}

func (t *Topic) MultiPublish(multiData [][]byte) (err error) {
	if len(multiData) == 0 {
		return nil
	}
	if err = t.Connect(); err != nil {
		return err
	}
	t.ExtendDeadline(t.parent.Deadline)
	var msgs []kafka.Message
	for _, d := range multiData {
		msgs = append(msgs, kafka.Message{Value: d})
	}
	if _, err = t.conn.WriteMessages(msgs...); err != nil {
		log.Fatal("failed to write messages:", err)
	}
	return nil
}

func (t *Topic) MultiConsume() (msgs [][]byte, err error) {
	if err = t.Connect(); err != nil {
		return nil, err
	}
	t.ExtendDeadline(t.parent.Deadline)

	batch := t.conn.ReadBatch(t.parent.MinBatchBytes, t.parent.MaxBatchBytes)

	var n int
	b := make([]byte, t.parent.MessageBytes)
	for {
		n, err = batch.Read(b)
		if err != nil {
			break
		}
		msgs = append(msgs, b[:n])
	}

	if err = batch.Close(); err != nil {
		return nil, err
	}
	return msgs, nil
}
