# Getting started

## TL;DR

You can run a docker compose environment with Kafka or RabbitMQ, PostgreSQL and pgarrow:
* Make sure you have docker, docker compose, and access to the internet (for docker images)
* Clone this repo locally
* Run ./docker-compose-test.yml
* Run psql against database src and insert into table t
* Run a second psql against dest and check that your data has arrived at the t table in the dest database

Commands:
```
# Run below without # to use rabbitmq instead of kafka
#export ARROW_MSGBUS=rabbitmq

mkdir -p ~/git/pgarrow
cd ~/git/pgarrow
[ -d .git ] && git pull || git clone https://github.com/MannemSolutions/pgarrow.git
bash ./docker-compose-tests.sh
docker exec -tiu postgres pgarrow-postgres-1 psql -d dest -c "select * from t;"
for ((i=i;i<10;i++)); do
  docker exec -tiu postgres pgarrow-postgres-1 psql -d src -c "insert into t select max(id+1), 'tst' from t;"
done
docker exec -tiu postgres pgarrow-postgres-1 psql -d dest -c "select * from t;"
```

### Known issues
Sometimes Kafka does not work as expected. Just restart the kafka and pgarrow pods, and you should be fine:
```
docker-compose down kafka pgarrowkafka kafkaarrowpg
docker-compose up kafka
sleep 5
docker-compose up pgarrowkafka kafkaarrowpg
```

## Bring your own environment

Below is described what is required to bring you own Postgres, and Kafka/RabbitMQ:

### Postgres

* wal_level needs to be set to logical
* a publication needs to be created
* a user with password (or other authentication) needs to be configured in postgres and pg_hba

Example:
```
# set wal_level to logical
psql -c 'alter system set wal_level = logical;'
pg_ctl restart

# Create publication
psql -c 'CREATE PUBLICATION pgarrow FOR ALL TABLES;'

# create a pgarrow user with proper permissions
psql -c "CREATE USER pgarrow WITH SUPERUSER, REPLICATION, PASSWORD 'pgarrow';"
# Please also setup pghba as required!!!
```

Check [example postgresql.conf](../config/postgresql.conf) and [example schema](../config/schema.sql) for more details.

### Kafka

Just create you own Kafka to be used.
Note: We have tested with a bare minimum setup, without TLS support (see #21) or authentication (see #20). Both are planned for release 1.0...

### RabbitMQ

Just create your own RabbitMQ to be used.
Notes:
* We have tested with a bare minimum setup, without TLS support (see #21) and no external authentication (see #20).
  * Both are planned for release 1.0...
* We used pgarrow for username and password in our tests. They can be set to whatever in the config file of pgarrow if you use other values.

### config

To run pgarrow you require a config file with proper options.
Below example config shows the minimal set of options that are probably required to be configured.
More options and more details on the exact options can be found [here](CONFIG.md).

```
---
kafka_config:
  brokers:
    - "kafka_hostname:9092"
pg_config:
  dsn:
    host: pg_hostname
    user: pg_user
    password: pg_password
rabbit_config:
  url: amqp://rabbit_user:rabbit_password@rabbit_hostname:5672/
```

Create a config file in ./config/pgarrow_local.yaml with the above content and replace:
- kafka_hostname with the full qualified hostname of the kafka broker
  - when working with multiple brokers you can them to the list
  - change port if needed
- pg_hostname with the full qualified hostname of the postgres database
- pg_user with the user for connecting to postgres (probably pgarrow)
- pg_password with the password of the user for connecting to postgres (probably pgarrow)
- You can set more dsn options for postgres if required
- rabbit_user with the user for connecting to RabbitMQ (using pgarrow is adviced)
- rabbit_password with the password for user for connecting to RabbitMQ
- rabbit_hostname with the full qualified hostname of the RabbitMQ host

### Download and run pgarrow

If Postgres and Kafka/RabbitMQ are probably setup, and you have defined a proper config file, you can start pgarrow with:
- When using kafka:
```
# Download pgarrow:
ARROW_BIN_VERSION=v0.1.4c
# For x86_64 (amd64):
curl -L https://github.com/MannemSolutions/pgarrow/releases/download/$ARROW_BIN_VERSION/arrow-$ARROW_BIN_VERSION-linux-amd64.tar.gz | tar -C ./bin -xz --include arrow
# For arm64:
curl -L https://github.com/MannemSolutions/pgarrow/releases/download/$ARROW_BIN_VERSION/arrow-$ARROW_BIN_VERSION-linux-arm64.tar.gz | tar -C ./bin -xz --include arrow

# KAFKA

# Start pgarrow to stream data from postgres (src database) to Kafka:
export PGDATABASE=src
export PGARROW_DIRECTION=pgarrowkafka
export PGARROW_CONFIGFILE=./config/pgarrow_local.yaml
nohup ./bin/arrow > "${PGARROW_DIRECTION}.out" 2> "${PGARROW_DIRECTION}.err" &

# Start pgarrow to stream data from Kafka to postgres (dest database):
export PGDATABASE=dest
export PGARROW_DIRECTION=kafkaarrowpg
export PGARROW_CONFIGFILE=./config/pgarrow_local.yaml
nohup ./bin/arrow > "${PGARROW_DIRECTION}.out" 2> "${PGARROW_DIRECTION}.err" &

# RABITMQ (ALTERNATE TO KAFKA)

# Start pgarrow to stream data from postgres (src database) to RabbitMQ
export PGDATABASE=src
export PGARROW_DIRECTION=pgarrowrabbit
export PGARROW_CONFIGFILE=./config/pgarrow_local.yaml
nohup ./bin/arrow > "${PGARROW_DIRECTION}.out" 2> "${PGARROW_DIRECTION}.err" &

# Start pgarrow to stream data from Kafka to postgres (dest database):
export PGDATABASE=dest
export PGARROW_DIRECTION=rabbitarrowpg
export PGARROW_CONFIGFILE=./config/pgarrow_local.yaml
nohup ./bin/arrow > "${PGARROW_DIRECTION}.out" 2> "${PGARROW_DIRECTION}.err" &
```
