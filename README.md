# pgarrow
Pgarrow is a tool developed for streaming Postgres changes through Kafka or RabbitMQ to keep tables in sync across database instances.
The pgarrow project creates 1 binary which can run 4 modes:
- pgarrowkafka (stream postgres cdc changes to Kafka)
- pgarrowrabbit (stream postgres cdc changes to RabbitMQ)
- kafkaarrowpg (apply cdc changes in postgres from Kafka)
- rabbitarrowpg (apply cdc changes in postgres from RabbitMQ)

To get started, please visit our [getting started](docs/TLDR.md) page...
