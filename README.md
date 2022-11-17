# pgarrow
Pgarrow is a tool developed for streaming Postgres changes through Kafka or RabbitMQ to keep tables in sync across database instances.
The pgarrow project creates 4 binaries:
- pgarrowkafka (stream postgres cdc changes to Kafka)
- pgarrowrabbit (stream postgres cdc changes to RabbitMQ)
- kafkaarrowpg (apply cdc changes in postgres from Kafka)
- rabbitarrowpg (apply cdc changes in postgres from RabbitMQ)
