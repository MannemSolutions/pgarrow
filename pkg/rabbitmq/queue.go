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

func (c *Queue) Connect() (err error) {
	if c.conn != nil {
		log.Info("RabbitMQ connection already initialized")
	} else if c.conn, err = amqp.Dial(c.config.Url); err != nil {
		return err
	}
	if c.channel != nil {
		log.Info("RabbitMQ channel already initialized")
	} else if c.channel, err = c.conn.Channel(); err != nil {
		return err
	}

	return nil
}

func (c *Queue) MustClose() {
	if err := c.Close(); err != nil {
		log.Fatalf("Error closing pg connection: %e", err)
	}
}

func (q Queue) Close() (err error) {
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
	return nil
}

func (c *Queue) CreateQueue() (err error) {
	if c.queue.Name != "" {
		log.Debugf("Queue already initialized")
		return nil
	}
	if err = c.Connect(); err != nil {
		return err
	}
	c.queue, err = c.channel.QueueDeclare(
		c.config.Queue,
		!c.config.Transient, // durable
		c.config.AutoDelete, // delete when unused
		false,               // exclusive
		false,               // no-wait
		nil,                 // arguments
	)
	return err
}

func (q Queue) Publish(data []byte) (err error) {
	qCtx, qCtxCancel := q.config.Context()
	defer qCtxCancel()

	return q.channel.PublishWithContext(qCtx,
		"",             // exchange
		q.config.Queue, // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        data,
		})
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

	var forever chan struct{}

	go func() {
		for delivery := range deliveries {
			log.Debugf("received a message of %d bytes", len(delivery.Body))
			PostProcessor(delivery.Body)
			delivery.Ack(false)
		}
	}()

	log.Info(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
	return nil
}
