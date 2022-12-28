package kafka

import (
	"errors"
	"github.com/segmentio/kafka-go"
	"net"
	"time"
)

type Topics map[string]*Topic

type Topic struct {
	name   string
	reader *kafka.Reader
	writer *kafka.Writer
	config *Config
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
	if t.reader = kafka.NewReader(t.config.ReaderConfig(t.name)); err != nil {
		log.Fatal("failed to dial leader:", err)
	}
	return nil
}

func (t *Topic) ConnectWriter() {
	if t.writer != nil {
		return
	}
	t.writer = &kafka.Writer{
		Addr:       kafka.TCP(t.config.Brokers...),
		Topic:      t.name,
		BatchBytes: int64(t.config.MaxBatchBytes),
		Async:      true,
	}
}

func (t *Topic) MustClose() {
	if err := t.Close(); err != nil {
		log.Fatalf("Error closing kafka connection: %e", err)
	}
}

func (t *Topic) Close() (err error) {
	if err = t.CloseReader(); err != nil {
		return err
	} else {
		return t.CloseWriter()
	}
}

func (t *Topic) CloseReader() (err error) {
	if t.reader == nil {
		return nil
	}
	if err = t.reader.Close(); err != nil {
		return err
	}
	t.reader = nil
	return nil
}

func (t *Topic) CloseWriter() (err error) {
	if t.writer == nil {
		return nil
	}
	if err = t.writer.Close(); err != nil {
		return err
	}
	t.writer = nil
	return nil
}

func (t *Topic) Publish(data []byte) (err error) {
	return t.MultiPublish([][]byte{data})
}

func (t *Topic) MultiPublish(multiData [][]byte) (err error) {
	if len(multiData) == 0 {
		return nil
	}
	var msgs []kafka.Message
	for _, d := range multiData {
		msgs = append(msgs, kafka.Message{Value: d})
	}
	for {
		// Use closure to defer tCtxCancel properly in a loop without leaking
		err = func() error {
			t.ConnectWriter()
			tCtx, tCtxCancel := t.config.Context()
			defer tCtxCancel()
			return t.writer.WriteMessages(tCtx, msgs...)
		}()
		switch err.(type) {
		case *net.OpError:
			log.Errorf("Kafka not available: %v", err)
			log.Infof("Retrying in 10 seconds")
			time.Sleep(10 * time.Second)
		case kafka.Error:
			log.Errorf("kafka error: (%T): %s", err, err)
			log.Infof("Retrying in 10 seconds")
			time.Sleep(10 * time.Second)
		case nil:
			log.Debugf("Data written to Kafka: %v", t.writer.Stats())
			return nil
		default:
			log.Errorf("failed to write messages: (%T): %s", err, err)
			return err
		}
	}
}

func (t *Topic) Consume() (m kafka.Message, err error) {
	if err = t.ConnectReader(); err != nil {
		return m, err
	}
	tCtx, tCtxCancel := t.config.Context()
	defer tCtxCancel()
	if m, err = t.reader.FetchMessage(tCtx); err != nil {
		return m, err
	}
	return m, nil
}

func (t *Topic) Commit(m kafka.Message) (err error) {
	tCtx, tCtxCancel := t.config.Context()
	defer tCtxCancel()
	return t.reader.CommitMessages(tCtx, m)
}

func processErrorUnWrapper(err error) error {
	mainError := err
	for {
		switch err.(type) {
		case *net.OpError, kafka.Error:
			return err
		case nil:
			return mainError
		default:
			err = errors.Unwrap(err)
		}
	}
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
			err = processErrorUnWrapper(err)
			switch err.(type) {
			case *net.OpError:
				log.Errorf("Kafka not available: %v", err)
			case kafka.Error:
				log.Errorf("Kafka error: %v", err)
			default:
				log.Errorf("I don't understand this error: (%T) -> %v", err, err)
				return err
			}
		} else if err = PostProcessor(msg.Value); err != nil {
			log.Debugf("PostProcessor error: %e", err)
			return err
		} else if err = t.reader.CommitMessages(ctx, msg); err != nil {
			log.Debugf("CommitMessages error: %e", err)
			return err
		}
	}
}
