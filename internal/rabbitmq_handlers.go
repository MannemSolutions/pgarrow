package internal

import (
	"github.com/mannemsolutions/pgarrrow/pkg/pg"
)

func HandlePgArrowRabbitMQ() {
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
	queue := config.RabbitMqConfig.NewQueue(config.RabbitMqConfig.Queue)
	defer queue.MustClose()
	if err = pgConn.StartRepl(); err != nil {
		log.Fatal(err)
	}
	if err = queue.CreateQueue(); err != nil {
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
		if err = queue.Publish(raw); err != nil {
			log.Fatal(err)
		}
	}
}

func HandleRabbitMQArrowPg() {
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
	log.Debug("Connecting to RabbitMQ")
	queue := config.RabbitMqConfig.NewQueue("stream")
	defer queue.MustClose()
	if err = queue.Process(pgConn.ProcessMsg); err != nil {
		log.Fatal(err)
	}
	log.Infof("Done")
}
