package rabbitmq

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

type Queues map[string]*Queue

type Queue struct {
	name    string
	config  *Config
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   amqp.Queue
}

func (q *Queue) Connect() (err error) {
	if q.conn != nil {
		log.Info("RabbitMQ connection already initialized")
	} else if q.conn, err = amqp.Dial(q.config.Url); err != nil {
		return err
	}
	if q.channel != nil {
		log.Info("RabbitMQ channel already initialized")
	} else if q.channel, err = q.conn.Channel(); err != nil {
		return err
	}

	return nil
}

func (q *Queue) MustClose() {
	if err := q.Close(); err != nil {
		log.Fatalf("Error closing pg connection: %e", err)
	}
}

func (q *Queue) Close() (err error) {
	if q.channel == nil {
		log.Debugf("channel not defined")
	} else if q.channel.IsClosed() {
		log.Debugf("channel was already closed")
	} else if err = q.channel.Close(); err != nil {
		return err
	}
	q.channel = nil

	if q.conn == nil {
		log.Debugf("connection not defined")
	} else if q.conn.IsClosed() {
		log.Debugf("connection was already closed")
	} else if err = q.conn.Close(); err != nil {
		return err
	}
	q.conn = nil
	q.queue = amqp.Queue{}
	return nil
}

func (q *Queue) CreateQueue() (err error) {
	if q.queue.Name != "" {
		log.Debugf("Queue already initialized")
		return nil
	}
	if err = q.Connect(); err != nil {
		return err
	}
	q.queue, err = q.channel.QueueDeclare(
		q.name,
		!q.config.Transient, // durable
		q.config.AutoDelete, // delete when unused
		false,               // exclusive
		false,               // no-wait
		nil,                 // arguments
	)
	return err
}

func (q Queue) Publish(data []byte) (err error) {
	qCtx, qCtxCancel := q.config.Context()
	defer qCtxCancel()

	err = q.channel.PublishWithContext(qCtx,
		"",     // exchange
		q.name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        data,
		})
	switch err.(type) {
	case *amqp.Error:
		log.Debugf("amqp error: %T: %v", err, err)
	case nil:
		return nil
	default:
		log.Debugf("Unknown error: %T: %v", err, err)
	}
	return err
}

func (q Queue) Process(PostProcessor func([]byte) error) (err error) {
	if err = q.CreateQueue(); err != nil {
		log.Fatal(err)
	}
	var deliveries <-chan amqp.Delivery
	deliveries, err = q.channel.Consume(
		q.queue.Name, // queue
		"",           // consumer
		false,        // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return err
	}

	for delivery := range deliveries {
		log.Debugf("received a message of %d bytes", len(delivery.Body))
		if err = PostProcessor(delivery.Body); err != nil {
			return err
		}
		if err = delivery.Ack(false); err != nil {
			return err
		}
	}

	return nil
}
