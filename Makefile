LATEST := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
MAJOR  := $(shell echo $(LATEST) | sed 's/v//' | cut -d. -f1)
MINOR  := $(shell echo $(LATEST) | sed 's/v//' | cut -d. -f2)
PATCH  := $(shell echo $(LATEST) | sed 's/v//' | cut -d. -f3)

.PHONY: patch minor major

patch:
	git tag v$(MAJOR).$(MINOR).$(shell echo $$(($(PATCH)+1))) && git push origin v$(MAJOR).$(MINOR).$(shell echo $$(($(PATCH)+1)))

minor:
	git tag v$(MAJOR).$(shell echo $$(($(MINOR)+1))).0 && git push origin v$(MAJOR).$(shell echo $$(($(MINOR)+1))).0

major:
	git tag v$(shell echo $$(($(MAJOR)+1))).0.0 && git push origin v$(shell echo $$(($(MAJOR)+1))).0.0
