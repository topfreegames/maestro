# maestro
# https://github.com/topfreegames/maestro
#
# Licensed under the MIT license:
# http://www.opensource.org/licenses/mit-license
# Copyright © 2017 Top Free Games <backend@tfgco.com>

MY_IP=`ifconfig | grep --color=none -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' | grep --color=none -Eo '([0-9]*\.){3}[0-9]*' | grep -v '127.0.0.1' | head -n 1`
TEST_PACKAGES=`find . -type f -name "*.go" ! \( -path "*vendor*" \) | sed -En "s/([^\.])\/.*/\1/p" | uniq`

.PHONY: plugins

setup: setup-hooks
	@go get -u github.com/golang/dep...
	@go get -u github.com/jteeuwen/go-bindata/...
	@go get -u github.com/wadey/gocovmerge
	@dep ensure

setup-hooks:
	@cd .git/hooks && ln -sf ./hooks/pre-commit.sh pre-commit

setup-ci:
	@go get github.com/mattn/goveralls
	@go get -u github.com/golang/dep/cmd/dep
	@go get github.com/onsi/ginkgo/ginkgo
	@go get -u github.com/wadey/gocovmerge
	@go get -u github.com/jteeuwen/go-bindata/...
	@dep ensure
	@dep ensure -update github.com/topfreegames/extensions

build:
	@mkdir -p bin && go build -o ./bin/maestro main.go

build-docker:
	@docker build -t maestro .

build-dev-room:
	@cd dev-room && docker build -t maestro-dev-room .
	@eval $(minikube docker-env -u)

delete-dev-room:
	@docker rmi -f maestro-dev-room

cross-build-linux-amd64:
	@env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ./bin/maestro-linux-amd64
	@chmod a+x ./bin/maestro-linux-amd64

assets:
	@go-bindata -o migrations/migrations.go -pkg migrations migrations/*.sql

migrate: assets
	@go run main.go migrate -c ./config/local.yaml

migrate-test: assets
	@go run main.go migrate -c ./config/test.yaml

deps: start-deps wait-for-pg 

start-deps:
	@echo "Starting dependencies using HOST IP of ${MY_IP}..."
	@env MY_IP=${MY_IP} docker-compose --project-name maestro up -d
	@sleep 10
	@echo "Dependencies started successfully."

start-deps-test:
	@echo "Starting dependencies using HOST IP of ${MY_IP}..."
	@env MY_IP=${MY_IP} docker-compose -f docker-compose-test.yaml --project-name maestro_test up -d
	@sleep 10
	@echo "Dependencies started successfully."

run:
	@go run main.go start

run-dev:
	@echo "serverUrl: http://localhost:8080" > ~/.maestro/config-local.yaml
	@env MAESTRO_OAUTH_ENABLED=false go run main.go start -v3

stop-deps:
	@env MY_IP=${MY_IP} docker-compose --project-name maestro down

wait-for-pg:
	@until docker exec maestro_postgres_1 pg_isready; do echo 'Waiting for Postgres...' && sleep 1; done
	@sleep 2

wait-for-test-pg:
	@until docker exec maestro_test_postgres_1 pg_isready; do echo 'Waiting for Postgres...' && sleep 1; done
	@sleep 2

deps-test: start-deps-test wait-for-test-pg drop-test migrate-test minikube

deps-test-ci: deps drop-test migrate-test minikube-ci

drop:
	@-docker exec maestro_postgres_1 psql -d postgres -h localhost -p 5432 -U postgres -c "SELECT pg_terminate_backend(pid.pid) FROM pg_stat_activity, (SELECT pid FROM pg_stat_activity where pid <> pg_backend_pid()) pid WHERE datname='maestro';"
	@docker exec maestro_postgres_1 psql -d postgres -h localhost -p 5432 -U postgres -f scripts/drop.sql > /dev/null
	@echo "Database created successfully!"

drop-test:
	@-docker exec maestro_test_postgres_1 psql -d postgres -h localhost -p 5432 -U postgres -c "SELECT pg_terminate_backend(pid.pid) FROM pg_stat_activity, (SELECT pid FROM pg_stat_activity where pid <> pg_backend_pid()) pid WHERE datname='maestro_test';"
	@docker exec maestro_test_postgres_1 psql -d postgres -h localhost -p 5432 -U postgres -f scripts/drop-test.sql > /dev/null
	@echo "Test Database created successfully!"

unit: unit-board clear-coverage-profiles unit-run gather-unit-profiles

clear-coverage-profiles:
	@find . -name '*.coverprofile' -delete

unit-board:
	@echo "\033[1;34m=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-\033[0m"
	@echo "\033[1;34m=         Unit Tests         -\033[0m"
	@echo "\033[1;34m=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-\033[0m"

unit-run:
	@ginkgo -tags unit -cover -r -randomizeAllSpecs -randomizeSuites -skipMeasurements ${TEST_PACKAGES}

gather-unit-profiles:
	@mkdir -p _build
	@echo "mode: count" > _build/coverage-unit.out
	@bash -c 'for f in $$(find . -name "*.coverprofile"); do tail -n +2 $$f >> _build/coverage-unit.out; done'

int integration: integration-board clear-coverage-profiles deps-test integration-run gather-integration-profiles

integration-board:
	@echo
	@echo "\033[1;34m=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-\033[0m"
	@echo "\033[1;34m=     Integration Tests      -\033[0m"
	@echo "\033[1;34m=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-\033[0m"

integration-run:
	@ MAESTRO_EXTENSIONS_PG_HOST=${MY_IP}                   \
    MAESTRO_EXTENSIONS_REDIS_URL=redis://${MY_IP}:6333    \
    ginkgo -tags integration -cover -r                    \
      -randomizeAllSpecs -randomizeSuites                 \
      -skipMeasurements worker api models controller; 		

int-ci: integration-board clear-coverage-profiles deps-test-ci integration-run gather-integration-profiles

gather-integration-profiles:
	@mkdir -p _build
	@echo "mode: count" > _build/coverage-integration.out
	@bash -c 'for f in $$(find . -name "*.coverprofile"); do tail -n +2 $$f >> _build/coverage-integration.out; done'

merge-profiles:
	@mkdir -p _build
	@gocovmerge _build/*.out > _build/coverage-all.out

test-coverage-func coverage-func: merge-profiles
	@echo
	@echo "\033[1;34m=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-\033[0m"
	@echo "\033[1;34mFunctions NOT COVERED by Tests\033[0m"
	@echo "\033[1;34m=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-\033[0m"
	@go tool cover -func=_build/coverage-all.out | egrep -v "100.0[%]"

test: unit int test-coverage-func 

test-ci: unit test-coverage-func 

test-coverage-html cover:
	@go tool cover -html=_build/coverage-all.out

rtfd:
	@rm -rf docs/_build
	@sphinx-build -b html -d ./docs/_build/doctrees ./docs/ docs/_build/html
	@open docs/_build/html/index.html

minikube:
	@/bin/bash ./scripts/start-minikube-if-not-yet.sh 

minikube-ci:
	@MAESTRO_TEST_CI=true /bin/bash ./scripts/start-minikube-if-not-yet.sh

work:
	@go run main.go worker -v3

clean-int-tests:
	@echo 'deleting maestro-test-* namespaces'
	@kubectl --context minikube get namespace | grep maestro-test- | awk '{print $$1}' | xargs kubectl --context minikube delete namespace
	@echo 'done'
	@echo 'deleting maestro-test-* nodes'
	@kubectl --context minikube get nodes | grep maestro-test- | awk '{print $$1}' | xargs kubectl --context minikube delete nodes
	@echo 'done'

generate-proto:
	@if [ -f eventforwarder/generated ]; then rm -r eventforwarder/generated; fi
	@mkdir -p plugins/grpc/generated
	@protoc -I plugins/grpc/protobuf/ plugins/grpc/protobuf/*.proto --go_out=plugins=grpc:plugins/grpc/generated
	@echo 'proto files created at plugins/grpc/generated'

plugins-linux:
	@go build -o bin/grpc.so -buildmode=plugin plugins/grpc/forwarder.go

plugins:
	@docker run -v $$(pwd)/:/go/src/github.com/topfreegames/maestro -ti golang bash -c "cd /go/src/github.com/topfreegames/maestro && go get github.com/golang/protobuf/protoc-gen-go google.golang.org/grpc golang.org/x/net/context && go build -o bin/grpc.so -buildmode=plugin plugins/grpc/forwarder.go"

.PHONY: mocks

mocks:
	@mockgen github.com/topfreegames/maestro/models PortChooser | sed 's/mock_models/mocks/' > mocks/port_chooser.go
	@echo 'port chooser mock on ./mocks/port_chooser.go'
