# Config

PgArrow requires config such as directions to Postgres, and RabbitMQ/Kafka.
Feeding this config is done through a config file and these docs describe the definition if this config file.

## Example

Below example config shows the options available to configure.
Detailed information on the exact options can be found below.

```
---
debug: true
kafka_config:
  brokers:
    - "localhost:9092"
  deadline: 1s
  max_batch_bytes: 1048576
  topic_prefix: pgarrow
pg_config:
  dsn:
    password: pgarrow
    user: postgres
    host: postgres
 slot_name: pgarrow
 standby_message_timeout: 10s 
rabbit_config:
  auto_delete: false
  deadline: 1s
  prefix: pgarrow
  transient: false
  url: amqp://arrow:arrow@rabbit:5672/
```
## Description of the options

### debug

The debug can be enabled for showing more information.
The option can be enabled in the config and/or on the commandline using the -x option.
It should only be enabled for debugging purposes for performance reasons, but also for data protection.

### kafka_config

The kafka_config option allows to set some parameters on the kafka connection.
When using RabitMQ this chapter is not used.
All options have 'sane defaults', but probably still require some config (probably kafka is not running locally).
The following options can be set:

#### brokers

This option configures a list of kafka endpoints (brokers), where every endpoint consists of a hostname/ip and a port separated by a colon.
The default contains only one item being "localhost:9092" and should probably be changed, unless kafka is running on the same host as pgarrow.
When multiple brokers are configured, pgarrow will fail over to the next when a broker is down.

#### deadline

The deadline option sets a timeout for kafka to return information during publishing and consumption.
The default of 1s should be fine, but if many Deadline exceeded errors in slow environments with big data chunks one could increase to see if it helps.

#### max_batch_bytes

Set how many bytes is written to Kafka in one go.
The default of 1MB usually is fine, but this value can be increased for more performance in high latency environments at the cost of memory consumption for pgarrow.

#### prefix

Current version of pgarrow uses only one topic, called stream, but that will probably be expanded when new features will be added.
Like a topic (copy) for a table to be copied while new dml is captured in stream.
And another for (re)creating the schema (ddl) from source.
All of these topics are hardcoded by name, but are prefixed with this option, which allows for running multiple instances of pgarrow on the same kafka.
Defaults to "pgarrow". Make sure it is set to the same value for both the writer and the reader.
Also make sure that it meets kafka topic name limitations:
- Only use symbols, letters, '.', '_' and '-'
- Don't use both '_' and '.' as combinations could collide in internal data structures
- Max 242 characters (as it is appended with 7 characters and must not exceed 249 characters)

See [kafka 3.3.1 source code](https://github.com/apache/kafka/blob/e23c59d00e687ff555d30bb4dc6c0cdec2c818ae/clients/src/main/java/org/apache/kafka/common/internals/Topic.java#L36) for more info.

### pg_config

The pg_config option allows for setting PostgreSQL connection configuration.
the following options are allowed:


#### dsn

The dsn option is a map of strings and can hold any option allowed for [github.com/jackc/pgx/v5/pgconn](https://github.com/jackc/pgx) which is most (if not all) of the [libpq keywords](https://www.postgresql.org/docs/12/libpq-connect.html#LIBPQ-PARAMKEYWORDS).
Usual settings include:
- host, which defaults to /tmp
- port, which defaults to 5432
- dbname, which defaults to the user
- user, which defaults to the linux user running pgarrow
- password, which defaults to "" (emptystring)

Note that replication=database (or other options) are automatically managed by pgarrow as required. No need but also no harm to set it...

#### slot_name

The slot_name option allows to set a name for the logical replication slot to be used.
Replication slots is a Postgres technique to keep track of the information already consumed from a replication channel.
Pgarrow uses a replication slot to make sure no data is lost in transit between Postgres and Kafka.
Note that replication slots are instance local, which makes a pgarrow setup with HA and connection fail over extra complex.
A solution has been designed, but not implemented yet.
Please leave a comment in [issue 18](https://github.com/MannemSolutions/pgarrow/issues/18) if you require such a setup.

#### standby_message_timeout

pgarrow expects a new message within the standby_message_timeout.
This parameter does not require any tuning unless postgres heartbeat configuration is configured with non-defaul setup.

### rabbit_config

#### auto_delete

Auto-delete queues that have had at least one consumer are deleted when the last consumer unsubscribes.
We currently see no benefit in enabling this feature, but decided to expose it for those who do.
Please refer to [RabbitMQ docs on queue properties](https://www.rabbitmq.com/queues.html#properties) for more info.

#### deadline

The deadline option sets a timeout for RabbitMQ to return information while publishing.
The consuming of queue messages are without deadline.
The default of 1s should be fine, but if many Deadline exceeded errors in slow environments with big data chunks one could increase to see if it helps.

#### prefix

Current version of pgarrow uses only one queue, called stream, but that will probably be expanded when new features will be added.
Like a topic (copy) for a table to be copied while new dml is captured in stream.
And another for (re)creating the schema (ddl) from source.
All of these topics are hardcoded by name, but are prefixed with the value of this option, which allows for running multiple instances of pgarrow on the same RabbitMQ.
This option defaults to "pgarrow". Make sure it is set to the same value for both the writer and the reader.
Also make sure that it meets RabbitMQ Queue name limitations: UTF-8 and max 248 characters (appended with "_stream" max 255).
Prefixes that start with "amq." are converted in a prefix starting with "pgarrow."

See [RabbitMQ docs on naming](https://www.rabbitmq.com/queues.html#names) for more info.

#### transient

Queues can either be durable or transient. 
Transient queues will be deleted on node boot. They therefore will not survive a node restart, by design. Messages in transient queues will also be discarded.
Durable queues will be recovered on node boot, including messages in them published as persistent. Messages published as transient will be discarded during recovery, even if they were stored in durable queues.

We currently see no benefit in using transient queues in production environments, but they might be used for demo purposes?), but decided to expose it for those who do.
Please refer to [RabbitMQ docs on queue durability](https://www.rabbitmq.com/queues.html#durability) for more info.

#### url

The url defines routing to the RabbitMQ endpoint.
The default (amqp://arrow:arrow@localhost:5672/) should probably be changed, unless kafka is running on the same host as pgarrow.
