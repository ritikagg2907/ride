SERVICES := user-service driver-service trip-service matching-service \
            surge-pricing-service payment-service notification-service admin-service

.PHONY: up down build-all test-all lint migrate-all $(SERVICES)

up:
	docker compose -f infra/docker-compose.yml up -d --build

down:
	docker compose -f infra/docker-compose.yml down -v

build-all:
	@for svc in $(SERVICES); do \
		echo "==> Building $$svc"; \
		cd services/$$svc && go build ./... && cd ../..; \
	done

test-all:
	@for svc in $(SERVICES); do \
		echo "==> Testing $$svc"; \
		cd services/$$svc && go test ./... && cd ../..; \
	done
	cd shared && go test ./...

lint:
	@for svc in $(SERVICES); do \
		cd services/$$svc && go vet ./... && cd ../..; \
	done

logs:
	docker compose -f infra/docker-compose.yml logs -f --tail=100

$(SERVICES):
	docker compose -f infra/docker-compose.yml up -d --build $@
