package internal

import (
	"github.com/mannemsolutions/pgarrrow/pkg/pg"
	"time"
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
	for {
		if err = pgConn.StartRepl(); err != nil {
			return err
		} else if err = pgConn.StreamTables(topic.Publish); err != nil {
			return err
		}
		t, pgErr := pgConn.NextTransactions()
		if pgErr != nil {
			if err = pgConn.Close(); err != nil {
				return err
			}
			log.Debug(pgErr)
			//return pgErr
		}
		if t.LSN == 0 {
			log.Debugln("received 0 transaction. Skipping")
			continue
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
	for {
		if err = pgConn.StartRepl(); err != nil {
			return err
		}
		t, pgErr := pgConn.NextTransactions()
		if pgErr != nil {
			if err = pgConn.Close(); err != nil {
				return err
			}
			log.Debug(pgErr)
			//return pgErr
		}
		if t.LSN == 0 {
			log.Debugln("received 0 transaction. Skipping")
			continue
		}
		raw, tErr := t.Dump()
		if tErr != nil {
			return tErr
		}
		if config.Debug {
			log.Debugf("Transaction (%d bytes): %s", len(raw), string(raw))
		}
		rErr := func() error {
			for {
				if err = queue.CreateQueue(); err != nil {
					log.Errorf("Unknown error: %v", err)
					return err
				} else if err = pgConn.StreamTables(queue.Publish); err != nil {
					return err
				}
				log.Debug("Queue created")
				if err = queue.Publish(raw); err != nil {
					log.Errorf("Error while publishing data")
					log.Infof("Retrying in 10 seconds")
					time.Sleep(10 * time.Second)
					if err = queue.Close(); err != nil {
						log.Errorf("Closing channel")
						return err
					}
					queue = config.RabbitMqConfig.NewQueue("stream")
				} else {
					log.Debug("Data published")
					return nil
				}
			}
		}()
		if rErr != nil {
			return rErr
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
	for {
		if err = queue.Process(pgConn.ProcessMsg); err != nil {
			return err
		}
		if err = queue.Close(); err != nil {
			return err
		}
	}
}
