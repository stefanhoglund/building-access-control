D2FLAGS := --theme 4 --pad 25
D2LAYOUT := elk

DIAGRAMS := context container component dynamic state deployment

FIGURES := $(addprefix docs/figures/,$(addsuffix .svg,$(DIAGRAMS)))

.PHONY: diagrams clean-diagrams \
	build run stop status seed \
	demo-allow demo-deny demo-decision-failure demo-actuator-failure \
	demo-normal-day demo-unauthorized demo-after-hours demo-rapid-repeat demo-burst \
	latency stress test test-race test-integration \
	docker-up docker-down docker-logs \
	docker-demo-allow docker-demo-deny docker-demo-after-hours \
	docker-demo-rapid-repeat docker-demo-normal-day docker-demo-decision-failure

diagrams: $(FIGURES)

docs/figures/%.svg: docs/diagrams/%.d2
	mkdir -p docs/figures
	d2 $(D2FLAGS) --layout $(D2LAYOUT) $< $@

clean-diagrams:
	rm -f docs/figures/*.svg

# --- operator reproducibility (Phase 5) -------------------------------------
# See scripts/run-local.sh for what each of these actually does and why it
# builds real binaries rather than using `go run`.

build:
	./scripts/run-local.sh build

run:
	./scripts/run-local.sh start

stop:
	./scripts/run-local.sh stop

status:
	./scripts/run-local.sh status

seed:
	./scripts/run-local.sh seed

demo-allow:
	./scripts/run-local.sh demo-allow

demo-deny:
	./scripts/run-local.sh demo-deny

demo-decision-failure:
	./scripts/run-local.sh demo-decision-failure

demo-actuator-failure:
	./scripts/run-local.sh demo-actuator-failure

demo-normal-day:
	./scripts/run-local.sh demo-normal-day

demo-unauthorized:
	./scripts/run-local.sh demo-unauthorized

demo-after-hours:
	./scripts/run-local.sh demo-after-hours

demo-rapid-repeat:
	./scripts/run-local.sh demo-rapid-repeat

demo-burst:
	./scripts/run-local.sh demo-burst

latency:
	go run ./cmd/latencybench -n 200 -interval 20ms

stress:
	go run ./cmd/stressbench -levels=1,10,50,100,200,400,800,1500,3000 -duration=2s

# --- verification (run after any change) ------------------------------------

test:
	gofmt -l . | grep -v '^docs/internal/' && exit 1 || true
	go vet ./...
	go test ./...

test-race:
	go test ./... -race

test-integration:
	go vet -tags=integration ./...
	go test -tags=integration ./test/integration/...

# --- Docker Compose deployment (docs/deployment.md) -------------------------
# The native workflow above (build/run/stop/...) is unaffected by any of
# this: both remain supported, as required.

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

docker-demo-allow:
	curl -s -X POST http://localhost:8081/scenario/run -H 'Content-Type: application/json' -d '{"scenario":"authorized-passage","seed":1}'; echo

docker-demo-deny:
	curl -s -X POST http://localhost:8081/scenario/run -H 'Content-Type: application/json' -d '{"scenario":"unauthorized-access","seed":1}'; echo

docker-demo-after-hours:
	curl -s -X POST http://localhost:8081/scenario/run -H 'Content-Type: application/json' -d '{"scenario":"after-hours-access","seed":1}'; echo

docker-demo-rapid-repeat:
	curl -s -X POST http://localhost:8081/scenario/run -H 'Content-Type: application/json' -d '{"scenario":"rapid-repeat-access","seed":1}'; echo

docker-demo-normal-day:
	curl -s -X POST http://localhost:8081/scenario/run -H 'Content-Type: application/json' -d '{"scenario":"normal-day","seed":42}'; echo

docker-demo-decision-failure:
	docker compose stop decision
	curl -s -X POST http://localhost:8081/scenario/run -H 'Content-Type: application/json' -d '{"scenario":"decision-service-failure","seed":1}'; echo
	sleep 1
	docker compose start decision
	sleep 1
	$(MAKE) docker-demo-allow