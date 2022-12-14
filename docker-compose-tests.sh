#!/bin/bash
set -e

COMPOSE_PROJDIR=$(basename $PWD)
ARROW_MSGBUS=${ARROW_MSGBUS:-kafka}
ARROW_BIN_SOURCE=${ARROW_BIN_SOURCE:-dockerhub}
ARROW_BIN_VERSION=${ARROW_BIN_VERSION:-v0.1.4}

#docker-compose down --remove-orphans #&& docker rmi ${COMPOSE_PROJDIR}-builder ${COMPOSE_PROJDIR}-stolon || echo new or partial install
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
  docker-compose up -d builder
  docker exec -ti ${COMPOSE_PROJDIR}-builder-1 /bin/bash -ic "cd /host && make build"
  cp "bin/arrow".* "./docker/arrow/"
  mv "./docker/arrow/arrow.aarch64" "./docker/arrow/arrow.arm64"
  cp config/pgarrow.yml "./docker/arrow/"
fi
docker-compose up -d "pgarrow${ARROW_MSGBUS}" "${ARROW_MSGBUS}arrowpg"
exit

docker-compose up -d builder
docker ps -a
assert primary 'host1'
assert primaries '[ host1 ]'
assert standbys '[ host2, host3 ]'

docker exec ${COMPOSE_PROJDIR}-postgres-2 /entrypoint.sh promote
assert primary ''
assert primaries '[ host1, host2 ]'
assert standbys '[ host3 ]'

docker exec ${COMPOSE_PROJDIR}-postgres-1 /entrypoint.sh rebuild
assert primary 'host2'
assert primaries '[ host2 ]'
assert standbys '[ host1, host3 ]'

echo "All is as expected"
