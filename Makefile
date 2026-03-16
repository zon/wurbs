.PHONY: infra rest socket install test push

IMAGE = zvonimir/wurbs
VERSION = $(shell cat ./version)

infra:
	cd infra && pulumi up --stack dev --yes

rest:
	air -build.cmd "go build -o ./tmp/rest ./server" -build.bin "./tmp/rest" -build.args_bin "--test" -build.include_dir "core,server,rest"

socket:
	air -build.cmd "go build -o ./tmp/socket ./socketserver" -build.bin "./tmp/socket" -build.args_bin "--test" -build.include_dir "core,socketserver,socket"

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
