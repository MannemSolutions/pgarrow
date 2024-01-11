#!/bin/bash
set -e

COMPOSE_PROJDIR=$(basename $PWD)
ARROW_MSGBUS=${ARROW_MSGBUS:-kafka}
ARROW_BIN_SOURCE=${ARROW_BIN_SOURCE:-build}
ARROW_BIN_VERSION=${ARROW_BIN_VERSION:-v0.1.4}

docker-compose down --remove-orphans 
docker-compose up -d "${ARROW_MSGBUS}" postgres
for ((i=0;i<10;i++)); do
  docker exec -u postgres ${COMPOSE_PROJDIR}-postgres-1 /usr/bin/pg_isready && break
  sleep 1
done
if ! docker exec -u postgres ${COMPOSE_PROJDIR}-postgres-1 /usr/bin/psql -tc "select datname from pg_database where datname='src'" | grep -q src; then
  docker exec -u postgres ${COMPOSE_PROJDIR}-postgres-1 /usr/bin/psql -c '\i /host/config/schema.sql'
  docker exec -u postgres ${COMPOSE_PROJDIR}-postgres-1 /usr/bin/psql -c '\i /host/config/inserts.sql'
fi
if [ "${ARROW_BIN_SOURCE}" = "github" ]; then
  mkdir -p bin
  curl -L https://github.com/MannemSolutions/pgarrow/releases/download/$ARROW_BIN_VERSION/arrow-$ARROW_BIN_VERSION-linux-amd64.tar.gz | tar -C ./bin -xz --include arrow && mv ./bin/arrow ./bin/arrow.amd64
  curl -L https://github.com/MannemSolutions/pgarrow/releases/download/$ARROW_BIN_VERSION/arrow-$ARROW_BIN_VERSION-linux-arm64.tar.gz | tar -C ./bin -xz --include arrow && mv ./bin/arrow ./bin/arrow.arm64
  cp config/pgarrow.yml "./docker/arrow/"
elif [ "${ARROW_BIN_SOURCE}" = "build" ]; then
  docker-compose up builder
  cp "bin/arrow" "./docker/arrow/"
  cp config/pgarrow.yml "./docker/arrow/"
fi
docker-compose up -d "pgarrow${ARROW_MSGBUS}" "${ARROW_MSGBUS}arrowpg"

LOGFILE=$(mktemp)
for ((i=0;i<60;i++)); do
  echo "Testrun ${i}"
  docker-compose up pgtester > $LOGFILE 
  grep -q ERROR $LOGFILE || break
  sleep 1
done
cat $LOGFILE
exit $(grep -c ERROR $LOGFILE)
