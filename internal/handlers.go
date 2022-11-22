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
		t, err := pgConn.NextTransaction()
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

func HandlePgArrowRabbitMQ() {
	initContext()
	config, err := NewConfig()
	if err != nil {
		initLogger("")
		log.Fatal(err)
	}
	initLogger(config.LogDest)
	pgConn := pg.NewConn(&config.PgConfig)
	defer pgConn.MustClose()
	topic := config.KafkaConfig.NewTopic("stream")
	defer topic.MustClose()

	var t pg.Transaction
	var msgs [][]byte
	for {
		msgs, err = topic.MultiConsume()
		if err != nil {
			log.Fatal(err)
		}
		for _, msg := range msgs {
			if t, err = pg.TransactionFromBytes(msg); err != nil {
				sql := t.Sql()
				log.Debugf("Running SQL: %s", sql)
				if err = pgConn.RunSQL(sql); err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}

func HandleKafkaArrowPg() {

}

func HandleRabbitMQArrowPg() {

}
