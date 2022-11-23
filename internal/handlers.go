package internal

import (
	"context"
	"github.com/jackc/pgx/v5/pgconn"
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

	var t pg.Transaction
	for {
		log.Debug("Consuming")
		m, kErr := topic.Consume()
		if kErr != nil {
			if kErr == context.DeadlineExceeded {
				log.Debugln("Deadline exceeded. Again...")
				continue
			}
			log.Fatal(kErr)
		}
		log.Debug("Processing messages")
		log.Debugf("Processing msg (%d bytes)", len(m.Value))
		if t, err = pg.TransactionFromBytes(m.Value); err != nil {
			log.Fatalf("Could not create a transaction from msg: %e", err)
		}
		sql := t.Sql()
		if err = pgConn.RunSQL(sql); err == nil {
			log.Debugf("succesfully ran %s", sql)
		} else if pgErr, ok := err.(*pgconn.PgError); !ok {
			log.Fatal(err)
		} else if pgErr.Code == "23505" {
			log.Infof("primary key violation, skipping: %s", sql)
		} else {
			log.Fatal(pgErr)
		}
		if err = topic.Commit(m); err != nil {
			log.Fatalf("Could not commit a msg to kafka: %e", err)
		}
	}
}

func HandleRabbitMQArrowPg() {

}
