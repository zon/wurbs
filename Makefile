.PHONY: infra rest socket install test push spec

IMAGE = zvonimir/wurbs
VERSION = $(shell cat ./version)

spec:
	asyncapi generate fromTemplate asyncapi.yaml @asyncapi/html-template --output ./docs/spec

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
