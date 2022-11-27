package kafka

import (
	"fmt"

	"github.com/segmentio/kafka-go"
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
	tCtx, tCtxCancel := t.parent.Context()
	defer tCtxCancel()
	if err = t.writer.WriteMessages(tCtx, msgs...); err != nil {
		log.Fatal("failed to write messages:", err)
	}
	return nil
}

func (t *Topic) Consume() (m kafka.Message, err error) {
	if err = t.ConnectReader(); err != nil {
		return m, err
	}
	tCtx, tCtxCancel := t.parent.Context()
	defer tCtxCancel()
	if m, err = t.reader.FetchMessage(tCtx); err != nil {
		return m, err
	}
	return m, nil
}

func (t *Topic) Commit(m kafka.Message) (err error) {
	tCtx, tCtxCancel := t.parent.Context()
	defer tCtxCancel()
	return t.reader.CommitMessages(tCtx, m)
}

func (t Topic) Process(PostProcessor func([]byte) error) (err error) {
	if err = t.ConnectReader(); err != nil {
		return err
	}

	var msg kafka.Message
	for {
		//tCtx, tCtxCancel := t.parent.Context()
		//defer tCtxCancel()
		if msg, err = t.reader.FetchMessage(ctx); err != nil {
			log.Debugf("FetchMessage error: %e", err)
			return err
		} else if err = PostProcessor(msg.Value); err != nil {
			log.Debugf("PostProcessor error: %e", err)
			return err
		} else if err = t.reader.CommitMessages(ctx, msg); err != nil {
			log.Debugf("CommitMessages error: %e", err)
			return err
		}
	}
}
