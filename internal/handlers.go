package internal

import (
	"github.com/mannemsolutions/pgarrrow/pkg/pg"
)

func MainHandler() {
	initContext()
	config, err := NewConfig()
	if err != nil {
		initLogger("")
		log.Fatal(err)
	}
	initLogger(config.LogDest)
	enableDebug(config.Debug)
	switch config.Direction {
	case "kafkaarrowpg":
		err = HandleKafkaArrowPg(config)
	case "pgarrowkafka":
		err = HandlePgArrowKafka(config)
	case "rabbitarrowpg":
		err = HandleRabbitArrowPg(config)
	case "pgarrowrabbit":
		err = HandlePgArrowRabbit(config)
	default:
		log.Fatalf("invalid value for direction '%s', please specify one of `-d pgarrowkafka`, "+
			"-d `kafkaarrowpg`, `-d pgarrowrabbit`, or `-d rabbitarrowpg`", config.Direction)
	}

	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Done")
}

func HandlePgArrowKafka(config Config) (err error) {
	log.Debug("Connecting to PostgreSQL")
	pgConn := pg.NewConn(&config.PgConfig)
	defer pgConn.MustClose()
	topic := config.KafkaConfig.NewTopic("stream")
	defer topic.MustClose()
	if err = pgConn.StartRepl(); err != nil {
		return err
	}
	for {
		t, pgErr := pgConn.NextTransactions()
		if pgErr != nil {
			return pgErr
		}
		raw, dErr := t.Dump()
		if dErr != nil {
			return dErr
		}
		if config.Debug {
			log.Debugf("Transaction (%d bytes): %s", len(raw), string(raw))
		}
		if err = topic.Publish(raw); err != nil {
			return err
		}
	}
}

func HandleKafkaArrowPg(config Config) (err error) {
	log.Debug("Connecting to PostgreSQL")
	pgConn := pg.NewConn(&config.PgConfig)
	defer pgConn.MustClose()
	log.Debug("Connecting to Kafka")
	topic := config.KafkaConfig.NewTopic("stream")
	defer topic.MustClose()

	return topic.Process(pgConn.ProcessMsg)
}

func HandlePgArrowRabbit(config Config) (err error) {
	log.Debug("Connecting to PostgreSQL")
	pgConn := pg.NewConn(&config.PgConfig)
	defer pgConn.MustClose()
	queue := config.RabbitMqConfig.NewQueue("stream")
	defer queue.MustClose()
	if err = pgConn.StartRepl(); err != nil {
		return err
	}
	if err = queue.CreateQueue(); err != nil {
		return err
	}
	for {
		t, pgErr := pgConn.NextTransactions()
		if pgErr != nil {
			return pgErr
		}
		raw, pgErr := t.Dump()
		if pgErr != nil {
			return pgErr
		}
		if config.Debug {
			log.Debugf("Transaction (%d bytes): %s", len(raw), string(raw))
		}
		if err = queue.Publish(raw); err != nil {
			return err
		}
	}
}

func HandleRabbitArrowPg(config Config) (err error) {
	log.Debug("Connecting to PostgreSQL")
	pgConn := pg.NewConn(&config.PgConfig)
	defer pgConn.MustClose()
	log.Debug("Connecting to RabbitMQ")
	queue := config.RabbitMqConfig.NewQueue("stream")
	defer queue.MustClose()
	return queue.Process(pgConn.ProcessMsg)
}
