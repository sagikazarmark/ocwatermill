# A Self-Documenting Makefile: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html

.PHONY: up
up: start ## Set up the environment

.PHONY: down
down: ## Destroy the environment
	docker-compose down
	rm -rf var/*

.PHONY: reset
reset: down up ## Reset the environment

docker-compose.override.yml:
	cp docker-compose.override.yml.dist docker-compose.override.yml

.PHONY: start
start: docker-compose.override.yml ## Start docker environment
	@ if [ docker-compose.override.yml -ot docker-compose.override.yml.dist ]; then diff -u docker-compose.override.yml docker-compose.override.yml.dist || (echo "!!! The distributed docker-compose.override.yml example changed. Please update your file accordingly (or at least touch it). !!!" && false); fi
	docker-compose up -d

.PHONY: stop
stop: ## Stop docker environment
	docker-compose stop

.PHONY: help
.DEFAULT_GOAL := help
help:
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
