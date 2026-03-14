.PHONY: infra rest socket install test push

infra:
	cd infra && pulumi up --stack dev --yes
