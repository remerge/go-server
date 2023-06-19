# https://www.gnu.org/software/make/manual/html_node/Choosing-the-Shell.html
SHELL = bash

# https://www.gnu.org/software/make/manual/html_node/Special-Variables.html
.DEFAULT_GOAL := help

.PHONY: help
help: ## generate help text from Makefile comments
	@grep -hE '^[a-zA-Z_0-9%-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# https://www.gnu.org/software/make/manual/html_node/Double_002dColon.html
.PHONY: install
install:: ## prepare environment and install dependencies
	@:

.PHONY: update
update:: ## update environment and dependencies
	@:

.PHONY: generate
generate:: ## generate documentation, configuration, schemas, etc
	@:

.PHONY: check
check:: ## run formatters and linters
	@:

.PHONY: test
test:: ## run unit and integration tests
	@:

.PHONY: build
build:: ## run build steps and create artifact
	@:

.PHONY: clean
clean:: ## remove build artifacts and caches
	@:

.PHONY: reset
reset: ## cleanup and reset repository to remote state
ifeq ($(FORCE),1)
	git reset --hard @{upstream}
	git clean -fdx
else
	@echo -e "\u001b[41mThis is a dangerous operation – use \`make reset FORCE=1' to execute\u001b[0m"
	@echo "Would execute \`git reset --hard @{upstream}'"
	@echo "Would execute \`git clean -fdx'"
	@git clean -ndx
endif

## copier

.PHONY: copier-copy
copier-copy: ## copy template without merging updates
	copier$(if $(FORCE), -f,)$(if $(REF), -r $(REF),) -w copy gh:remerge/template .

.PHONY: copier-update
copier-update: ## update project from copier template
	copier$(if $(FORCE), -f,)$(if $(REF), -r $(REF),) -w update
update:: copier-update

## pre-commit

.git/hooks/pre-commit:
	make pre-commit-install

.PHONY: pre-commit-install
pre-commit-install: ## install pre-commit hook
	pre-commit install -t pre-commit -t prepare-commit-msg -t commit-msg
install:: pre-commit-install

.PHONY: pre-commit-check
pre-commit-check: ## run pre commit hooks
pre-commit-check: .git/hooks/pre-commit
	pre-commit run --all-files
check:: pre-commit-check

.PHONY: pre-commit-clean
pre-commit-clean: ## remove pre-commit and cached repositories
	pre-commit uninstall
	pre-commit clean
clean:: pre-commit-clean

-include *.mk
