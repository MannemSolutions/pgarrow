#!/bin/bash
set -e

function assert() {
  TST=$((TST+1))
  EP=$1
  EXPECTED=$2
  if [ -e pgroute66.crt ]; then
    RESULT=$(curl --cacert pgroute66.crt "https://localhost:8443/v1/${EP}" | xargs)
  else
    RESULT=$(curl "http://localhost:8080/v1/${EP}" | xargs)
  fi
  if [ "${RESULT}" = "${EXPECTED}" ]; then
    echo "test${TST}: OK"
  else
    echo "test${TST}: EROR: expected '${EXPECTED}', but got '${RESULT}'"
    docker-compose logs pgroute66 postgres
    return 1
  fi
}

TST=0
COMPOSE_PROJDIR=$(basename $PWD)

#docker-compose down --remove-orphans #&& docker rmi ${COMPOSE_PROJDIR}_builder ${COMPOSE_PROJDIR}_stolon || echo new or partial install
docker-compose up -d builder
docker exec ${COMPOSE_PROJDIR}_builder_1 /bin/bash -ic 'cd /host && make build'
for F in pgarrowkafka kafkaarrowpg; do
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
