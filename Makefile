GO ?= go
PACKAGES ?= ./...
BENCH ?= .
BENCHTIME ?= 5s
COUNT ?= 5

LIMACTL ?= limactl
LIMA_INSTANCE_PREFIX ?= facts
LIMA_GO_VERSION ?= 1.25.0
LIMA_GO_SERIES ?= 1.25
LIMA_GOARCH ?= arm64
LIMA_CPUS ?= 6
LIMA_MEMORY ?= 10
LIMA_DISK ?= 80

LIMA_DEV_TEMPLATE ?= ubuntu-lts
LIMA_DEV_INSTANCE ?= $(LIMA_INSTANCE_PREFIX)-dev
LIMA_DOCKER_INSTANCE ?= $(LIMA_INSTANCE_PREFIX)-docker
LIMA_FREEBSD_INSTANCE ?= $(LIMA_INSTANCE_PREFIX)-freebsd

LIMA_DEV_FLAGS ?= --vm-type=vz --rosetta --mount-writable --cpus $(LIMA_CPUS) --memory $(LIMA_MEMORY) --disk $(LIMA_DISK)
LIMA_DOCKER_FLAGS ?= --vm-type=vz --rosetta --mount-writable --cpus $(LIMA_CPUS) --memory $(LIMA_MEMORY) --disk $(LIMA_DISK)
LIMA_LINUX_VM_FLAGS ?= --vm-type=vz --mount-writable --cpus 2 --memory 4 --disk 40
LIMA_LINUX_PLAIN_FLAVORS ?= opensuse oraclelinux rocky almalinux
LIMA_FREEBSD_FLAGS ?= --mount-none --cpus 2 --memory 4 --disk 40

LIMA_GO_CONTAINER_IMAGES ?= golang:1.26-bookworm golang:1.26-alpine
LIMA_DISTRO_IMAGES ?= debian:12-slim ubuntu:24.04 archlinux:latest oraclelinux:9
LIMA_LINUX_FLAVORS ?= ubuntu-lts debian fedora opensuse oraclelinux rocky almalinux alpine archlinux
LIMA_CROSS_TARGETS ?= linux/amd64 linux/arm64 windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 freebsd/amd64

LIMA_LINUX_BINARY ?= dist/facts-linux-$(LIMA_GOARCH)
LIMA_FREEBSD_BINARY ?= dist/facts-freebsd-$(LIMA_GOARCH)

VERSION := $(shell sed -n 's/^const Version = "\(.*\)"$$/\1/p' internal/engine/core.go)
PREFIX ?= /usr/local
DESTDIR ?=
DIST_DIR ?= dist
DIST_TARGETS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64
SHA256 := $(shell command -v sha256sum >/dev/null 2>&1 && echo "sha256sum" || echo "shasum -a 256")

.PHONY: test race bench bench-stable build clean dist install
.PHONY: lima-help lima-all lima-ci lima-dev-start lima-dev-bootstrap lima-dev-checks lima-dev-test
.PHONY: lima-build-linux-binary lima-cross-compile lima-docker-start lima-docker-go-containers
.PHONY: lima-docker-build-amd64 lima-docker-distro-facts lima-docker-workloads
.PHONY: lima-linux-flavors lima-linux-flavor-smoke lima-build-freebsd-binary lima-freebsd-start
.PHONY: lima-freebsd-smoke lima-stop

test:
	$(GO) test $(PACKAGES)

race:
	$(GO) test -race $(PACKAGES)

bench:
	$(GO) test -run '^$$' -bench '$(BENCH)' -benchmem $(PACKAGES)

bench-stable:
	$(GO) test -run '^$$' -bench '$(BENCH)' -benchtime $(BENCHTIME) -count $(COUNT) -benchmem $(PACKAGES)

build:
	$(GO) build -o facts ./cmd/facts

# dist builds checksummed release archives facts-$(VERSION)-<os>-<arch> for
# every supported os/arch pair. The version is embedded in the binary
# (internal/engine.Version) and reported by `facts --version`.
dist:
	@set -e; \
	mkdir -p $(DIST_DIR); \
	for target in $(DIST_TARGETS); do \
		goos=$${target%/*}; goarch=$${target#*/}; \
		name="facts-$(VERSION)-$$goos-$$goarch"; \
		bin=facts; \
		if [ "$$goos" = windows ]; then bin=facts.exe; fi; \
		staging="$(DIST_DIR)/$$name"; \
		rm -rf "$$staging"; \
		mkdir -p "$$staging"; \
		echo "building $$name"; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch $(GO) build -trimpath -o "$$staging/$$bin" ./cmd/facts; \
		if [ "$$goos" = windows ]; then \
			rm -f "$(DIST_DIR)/$$name.zip"; \
			(cd $(DIST_DIR) && zip -q -r "$$name.zip" "$$name"); \
		else \
			tar -czf "$(DIST_DIR)/$$name.tar.gz" -C $(DIST_DIR) "$$name"; \
		fi; \
		rm -rf "$$staging"; \
	done; \
	(cd $(DIST_DIR) && rm -f SHA256SUMS && $(SHA256) facts-$(VERSION)-*.tar.gz facts-$(VERSION)-*.zip > SHA256SUMS); \
	cat $(DIST_DIR)/SHA256SUMS

# install builds the host binary and installs it under PREFIX (default
# /usr/local), honoring DESTDIR for staged installs.
install: build
	install -d "$(DESTDIR)$(PREFIX)/bin"
	install -m 0755 facts "$(DESTDIR)$(PREFIX)/bin/facts"

clean:
	rm -f facts
	rm -rf $(DIST_DIR)/facts-$(VERSION)-* $(DIST_DIR)/SHA256SUMS

lima-help:
	@printf '%s\n' 'Lima targets:'
	@printf '%s\n' '  make lima-ci              # Linux VM checks/tests + Docker workload matrix + cross compile'
	@printf '%s\n' '  make lima-all             # lima-ci + per-distro Linux VM smoke + FreeBSD smoke'
	@printf '%s\n' '  make lima-dev-test        # Go checks, tests, race, build, and CLI smoke in Ubuntu Lima'
	@printf '%s\n' '  make lima-docker-workloads# CI-like Go container and Linux distro fact smoke tests'
	@printf '%s\n' '  make lima-linux-flavors   # Smoke the built Linux CLI in each Lima Linux flavor'
	@printf '%s\n' '  make lima-freebsd-smoke   # Copy-built FreeBSD binary into Lima FreeBSD and smoke it'
	@printf '%s\n' 'Set LIMA_KEEP_RUNNING=1 to leave flavor/FreeBSD VMs running after smoke tests.'

lima-all: lima-ci lima-linux-flavors lima-freebsd-smoke

lima-ci: lima-dev-checks lima-dev-test lima-docker-workloads lima-cross-compile

lima-dev-start:
	@if $(LIMACTL) list --format '{{.Name}}' | grep -qx '$(LIMA_DEV_INSTANCE)'; then \
		$(LIMACTL) start '$(LIMA_DEV_INSTANCE)'; \
	else \
		$(LIMACTL) start 'template:$(LIMA_DEV_TEMPLATE)' --name '$(LIMA_DEV_INSTANCE)' $(LIMA_DEV_FLAGS); \
	fi

lima-dev-bootstrap: lima-dev-start
	$(LIMACTL) shell '$(LIMA_DEV_INSTANCE)' -- bash -lc 'set -eu; \
		if command -v apt-get >/dev/null 2>&1; then \
			sudo apt-get update; \
			sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates curl git make; \
		fi; \
		arch="$$(uname -m)"; \
		case "$$arch" in \
			aarch64|arm64) goarch=arm64 ;; \
			x86_64|amd64) goarch=amd64 ;; \
			*) echo "unsupported Linux VM arch: $$arch" >&2; exit 1 ;; \
		esac; \
		go_ok=0; \
		if command -v go >/dev/null 2>&1; then \
			go_version="$$(go version)"; \
			case "$$go_version" in \
				*"go$(LIMA_GO_SERIES)."*) printf "%s\n" "$$go_version"; go_ok=1 ;; \
			esac; \
		fi; \
		if [ "$$go_ok" = 1 ]; then \
			:; \
		else \
			tmp="$$(mktemp -d)"; \
			curl -fsSL "https://go.dev/dl/go$(LIMA_GO_VERSION).linux-$$goarch.tar.gz" -o "$$tmp/go.tgz"; \
			sudo rm -rf /usr/local/go; \
			sudo tar -C /usr/local -xzf "$$tmp/go.tgz"; \
			sudo ln -sf /usr/local/go/bin/go /usr/local/bin/go; \
			sudo ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt; \
			rm -rf "$$tmp"; \
			go version; \
		fi'

lima-dev-checks: lima-dev-bootstrap
	$(LIMACTL) shell '$(LIMA_DEV_INSTANCE)' -- bash -lc 'set -eu; \
		cd "$(CURDIR)"; \
		files="$$(gofmt -l $$(git ls-files "*.go"))"; \
		if [ -n "$$files" ]; then \
			printf "The following Go files need gofmt:\n%s\n" "$$files"; \
			exit 1; \
		fi; \
		go mod tidy; \
		git diff --exit-code -- go.mod go.sum; \
		go vet ./...'

lima-dev-test: lima-dev-bootstrap
	$(LIMACTL) shell '$(LIMA_DEV_INSTANCE)' -- bash -lc 'set -eu; \
		cd "$(CURDIR)"; \
		export GOCACHE="$${HOME}/.cache/go-build"; \
		export GOMODCACHE="$${HOME}/.cache/go-mod"; \
		mkdir -p "$$GOCACHE" "$$GOMODCACHE"; \
		make test; \
		make race; \
		make build; \
		./facts --json os.name kernel virtual'

lima-build-linux-binary: lima-dev-bootstrap
	$(LIMACTL) shell '$(LIMA_DEV_INSTANCE)' -- bash -lc 'set -eu; \
		cd "$(CURDIR)"; \
		mkdir -p dist; \
		CGO_ENABLED=0 GOOS=linux GOARCH=$(LIMA_GOARCH) go build -o "$(LIMA_LINUX_BINARY)" ./cmd/facts'

lima-cross-compile: lima-dev-bootstrap
	@for target in $(LIMA_CROSS_TARGETS); do \
		goos=$${target%/*}; \
		goarch=$${target#*/}; \
		echo "==> cross compile $$goos/$$goarch"; \
		$(LIMACTL) shell '$(LIMA_DEV_INSTANCE)' -- bash -lc "set -eu; cd '$(CURDIR)'; CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go build ./..." || exit $$?; \
	done

lima-docker-start:
	@if $(LIMACTL) list --format '{{.Name}}' | grep -qx '$(LIMA_DOCKER_INSTANCE)'; then \
		$(LIMACTL) start '$(LIMA_DOCKER_INSTANCE)'; \
	else \
		$(LIMACTL) start template:docker --name '$(LIMA_DOCKER_INSTANCE)' $(LIMA_DOCKER_FLAGS); \
	fi

lima-docker-go-containers: lima-docker-start
	@for image in $(LIMA_GO_CONTAINER_IMAGES); do \
		echo "==> Go workload container $$image"; \
		$(LIMACTL) shell '$(LIMA_DOCKER_INSTANCE)' -- sh -lc "set -eu; cd '$(CURDIR)'; docker run --rm -e CI=true -v \"\$$PWD:/workspace\" -w /workspace $$image sh -c 'go test ./... && go run ./cmd/facts --json os.name kernel virtual'" || exit $$?; \
	done

lima-docker-build-amd64: lima-docker-start
	$(LIMACTL) shell '$(LIMA_DOCKER_INSTANCE)' -- sh -lc "set -eu; \
		cd '$(CURDIR)'; \
		mkdir -p dist; \
		docker run --rm -e CI=true -v \"\$$PWD:/workspace\" -w /workspace golang:1.26-bookworm \
			sh -c 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/facts-linux-amd64 ./cmd/facts'"

lima-docker-distro-facts: lima-docker-build-amd64
	@for image in $(LIMA_DISTRO_IMAGES); do \
		case "$$image" in \
			debian:*) expected_id=debian ;; \
			ubuntu:*) expected_id=ubuntu ;; \
			archlinux:*) expected_id=arch ;; \
			oraclelinux:*) expected_id=ol ;; \
			*) echo "missing expected os.distro.id for $$image" >&2; exit 2 ;; \
		esac; \
		echo "==> distro fact smoke $$image"; \
		$(LIMACTL) shell '$(LIMA_DOCKER_INSTANCE)' -- sh -lc "set -eu; cd '$(CURDIR)'; out=\$$(docker run --rm --platform linux/amd64 -e CI=true -v \"\$$PWD/dist/facts-linux-amd64:/usr/local/bin/facts:ro\" $$image /usr/local/bin/facts --json os.name os.family os.distro.id os.distro.description os.release.major os.distro.release.major kernel virtual); printf '%s\n' \"\$$out\"; printf '%s\n' \"\$$out\" | grep -Eq '\"kernel\"[[:space:]]*:[[:space:]]*\"Linux\"'; printf '%s\n' \"\$$out\" | grep -Eq '\"os.distro.id\"[[:space:]]*:[[:space:]]*\"$$expected_id\"'" || exit $$?; \
	done

lima-docker-workloads: lima-docker-go-containers lima-docker-distro-facts

lima-linux-flavors: lima-build-linux-binary
	@for flavor in $(LIMA_LINUX_FLAVORS); do \
		$(MAKE) lima-linux-flavor-smoke LIMA_FLAVOR=$$flavor LIMA_SKIP_BUILD=1 || exit $$?; \
	done

lima-linux-flavor-smoke: $(if $(LIMA_SKIP_BUILD),,lima-build-linux-binary)
	@test -n "$(LIMA_FLAVOR)" || (echo "set LIMA_FLAVOR=<template>, e.g. ubuntu-lts" >&2; exit 2)
	@set -e; \
		instance="$(LIMA_INSTANCE_PREFIX)-linux-$$(printf '%s' '$(LIMA_FLAVOR)' | tr '/:' '--')"; \
		flags="$(LIMA_LINUX_VM_FLAGS)"; \
		case " $(LIMA_LINUX_PLAIN_FLAVORS) " in \
			*" $(LIMA_FLAVOR) "*) flags="--plain $$flags" ;; \
		esac; \
		cleanup() { \
			if [ "$(LIMA_KEEP_RUNNING)" != "1" ]; then \
				$(LIMACTL) stop "$$instance" >/dev/null 2>&1 || true; \
			fi; \
		}; \
		trap cleanup EXIT; \
		if $(LIMACTL) list --format '{{.Name}}' | grep -qx "$$instance"; then \
			$(LIMACTL) start "$$instance"; \
		else \
			$(LIMACTL) start "template:$(LIMA_FLAVOR)" --name "$$instance" $$flags; \
		fi; \
		echo "==> Lima Linux flavor smoke $(LIMA_FLAVOR) ($$instance)"; \
		$(LIMACTL) shell "$$instance" -- sh -c 'cat > /tmp/facts' < '$(LIMA_LINUX_BINARY)'; \
		$(LIMACTL) shell "$$instance" -- chmod +x /tmp/facts; \
		$(LIMACTL) shell "$$instance" -- /tmp/facts --json os.name os.family os.distro.id kernel virtual

lima-build-freebsd-binary: lima-dev-bootstrap
	$(LIMACTL) shell '$(LIMA_DEV_INSTANCE)' -- bash -lc 'set -eu; \
		cd "$(CURDIR)"; \
		mkdir -p dist; \
		CGO_ENABLED=0 GOOS=freebsd GOARCH=$(LIMA_GOARCH) go build -o "$(LIMA_FREEBSD_BINARY)" ./cmd/facts'

lima-freebsd-start:
	@if $(LIMACTL) list --format '{{.Name}}' | grep -qx '$(LIMA_FREEBSD_INSTANCE)'; then \
		$(LIMACTL) start '$(LIMA_FREEBSD_INSTANCE)'; \
	else \
		$(LIMACTL) start template:freebsd --name '$(LIMA_FREEBSD_INSTANCE)' $(LIMA_FREEBSD_FLAGS); \
	fi

lima-freebsd-smoke: lima-build-freebsd-binary lima-freebsd-start
	@set -e; \
		if [ "$(LIMA_KEEP_RUNNING)" != "1" ]; then \
			trap '$(LIMACTL) stop "$(LIMA_FREEBSD_INSTANCE)" >/dev/null 2>&1 || true' EXIT; \
		fi; \
		$(LIMACTL) copy '$(LIMA_FREEBSD_BINARY)' '$(LIMA_FREEBSD_INSTANCE):/tmp/facts'; \
		$(LIMACTL) copy tools/freebsd-release-gate.sh '$(LIMA_FREEBSD_INSTANCE):/tmp/freebsd-release-gate.sh'; \
		$(LIMACTL) shell '$(LIMA_FREEBSD_INSTANCE)' -- chmod +x /tmp/facts; \
		$(LIMACTL) shell '$(LIMA_FREEBSD_INSTANCE)' -- sh /tmp/freebsd-release-gate.sh /tmp/facts

lima-stop:
	@for instance in '$(LIMA_DEV_INSTANCE)' '$(LIMA_DOCKER_INSTANCE)' '$(LIMA_FREEBSD_INSTANCE)'; do \
		$(LIMACTL) stop "$$instance" >/dev/null 2>&1 || true; \
	done
	@for flavor in $(LIMA_LINUX_FLAVORS); do \
		instance="$(LIMA_INSTANCE_PREFIX)-linux-$$(printf '%s' "$$flavor" | tr '/:' '--')"; \
		$(LIMACTL) stop "$$instance" >/dev/null 2>&1 || true; \
	done
