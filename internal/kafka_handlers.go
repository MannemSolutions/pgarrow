package internal

import (
	"github.com/mannemsolutions/pgarrrow/pkg/pg"
)

func HandlePgArrowKafka() {
	initContext()
	config, err := NewConfig()
	if err != nil {
		initLogger("")
		log.Fatal(err)
	}
	initLogger(config.LogDest)
	enableDebug(config.Debug)
	pgConn := pg.NewConn(&config.PgConfig)
	defer pgConn.MustClose()
	topic := config.KafkaConfig.NewTopic("stream")
	defer topic.MustClose()
	if err = pgConn.StartRepl(); err != nil {
		log.Fatal(err)
	}
	for {
		t, err := pgConn.NextTransactions()
		if err != nil {
			log.Fatal(err)
		}
		raw, err := t.Dump()
		if config.Debug {
			log.Debugf("Transaction (%d bytes): %s", len(raw), string(raw))
		}
		if err = topic.Publish(raw); err != nil {
			log.Fatal(err)
		}
	}
}

func HandleKafkaArrowPg() {
	initContext()
	config, err := NewConfig()
	if err != nil {
		initLogger("")
		log.Fatal(err)
	}
	initLogger(config.LogDest)
	enableDebug(config.Debug)
	log.Debug("Connecting to PostgreSQL")
	pgConn := pg.NewConn(&config.PgConfig)
	defer pgConn.MustClose()
	log.Debug("Connecting to Kafka")
	topic := config.KafkaConfig.NewTopic("stream")
	defer topic.MustClose()

	if err = topic.Process(pgConn.ProcessMsg); err != nil {
		log.Fatal(err)
	}
	log.Infof("Done")
}
