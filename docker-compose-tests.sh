#!/bin/bash
set -e

COMPOSE_PROJDIR=$(basename $PWD)
ARROW_MSGBUS=${ARROW_MSGBUS:-kafka}

#docker-compose down --remove-orphans #&& docker rmi ${COMPOSE_PROJDIR}_builder ${COMPOSE_PROJDIR}_stolon || echo new or partial install
docker-compose up -d "${ARROW_MSGBUS}" postgres builder
docker exec -u postgres ${COMPOSE_PROJDIR}_postgres_1 /usr/bin/psql -tc "select datname from pg_database where datname='src'" | grep -q src || docker exec -u postgres ${COMPOSE_PROJDIR}_postgres_1 /usr/bin/psql -c '\i /host/config/schema.sql'
docker exec -ti ${COMPOSE_PROJDIR}_builder_1 /bin/bash -ic "cd /host && make build_${ARROW_MSGBUS}"
for F in "pgarrow${ARROW_MSGBUS}" "${ARROW_MSGBUS}arrowpg"; do
  cp "bin/${F}".* "./docker/$F/"
  mv "./docker/${F}/${F}.aarch64" "./docker/${F}/${F}.arm64"
  cp config/pgarrow.yml "./docker/$F/"
  docker-compose up -d "$F"
done
exit

docker-compose up -d builder
docker ps -a
assert primary 'host1'
assert primaries '[ host1 ]'
assert standbys '[ host2, host3 ]'

docker exec ${COMPOSE_PROJDIR}_postgres_2 /entrypoint.sh promote
assert primary ''
assert primaries '[ host1, host2 ]'
assert standbys '[ host3 ]'

docker exec ${COMPOSE_PROJDIR}_postgres_1 /entrypoint.sh rebuild
assert primary 'host2'
assert primaries '[ host2 ]'
assert standbys '[ host1, host3 ]'

echo "All is as expected"
