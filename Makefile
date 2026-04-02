.PHONY: infra rest socket install test push spec generate

IMAGE = zvonimir/wurbs
VERSION = $(shell cat ./version)

spec: spec-asyncapi spec-openapi
	@echo "Generated API documentation in ./docs/"

spec-asyncapi:
	asyncapi generate fromTemplate specs/socket.yaml @asyncapi/html-template --output ./docs/socket --force-write

spec-openapi:
	redoc-cli bundle specs/rest.yaml --output ./docs/rest/index.html

generate: generate-openapi generate-asyncapi
	@echo "Generated API types in ./gen/"

generate-openapi:
	mkdir -p ./gen/openapi
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
		-generate=types \
		-package=openapi \
		-o ./gen/openapi/types.go \
		specs/rest.yaml
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
		-generate=gin \
		-package=openapi \
		-o ./gen/openapi/server.go \
		specs/rest.yaml

generate-asyncapi:
	mkdir -p ./gen/asyncapi
	asyncapi validate specs/socket.yaml
	@echo "AsyncAPI types are manually maintained in gen/asyncapi/types.go"

infra:
	cd infra && pulumi up --stack dev --yes

rest:
	air -c rest/.air.toml -- --test

socket:
	air -c socket/.air.toml -- --test

install:
	go install ./wurbctl

test:
	go test ./...

push:
	cat .docker-token | docker login -u zvonimir --password-stdin
	docker build --platform linux/amd64 --tag $(IMAGE):$(VERSION) .
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest
