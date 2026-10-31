SHELL := /usr/bin/env bash -euo pipefail

.PHONY: checkout

## checkout: pull & checkout a factory feature branch (usage: make checkout [job=001-parent-direct-approve]); defaults to main
checkout:
	@bash factory/checkout.sh "$(job)"
