bootstrap:
	docker compose up -d && bash ./scripts/etcd.bash

protoc:
	protoc --go_out=. \
		--go_opt=paths=source_relative \
        --go-grpc_out=. \
        --go-grpc_opt=paths=source_relative \
        app/proto/*.proto


otel:
	docker compose up -d && bash ./scripts/uptrace.bash

uptrace:
	uptrace --config=config/uptrace.yml serve

uptrace-reset:
	uptrace --config=uptrace.yml ch reset

status:
	etcdctl --endpoints=http://localhost:2379 member list