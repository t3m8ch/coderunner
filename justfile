dev:
    REDIS_HOST=localhost:6379 REDIS_DB=0 MINIO_ENDPOINT=localhost:9000 MINIO_ACCESS_KEY=coderunner_user MINIO_SECRET_KEY=abobaaboba123 air -c .air.toml

minio:
    docker run --rm -p 9000:9000 -p 9001:9001 --name minio -e "MINIO_ROOT_USER=coderunner_user" -e "MINIO_ROOT_PASSWORD=abobaaboba123" -v ./minio/data:/data minio/minio server /data --console-address ":9001"

redis:
    docker run --rm --name dragonfly --ulimit memlock=-1 --network=host docker.dragonflydb.io/dragonflydb/dragonfly

redis-cli:
    docker exec -it dragonfly redis-cli
