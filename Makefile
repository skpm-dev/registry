LATEST := $(or $(shell git tag --sort=-version:refname 2>/dev/null | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+' | head -1),v0.0.0)
MAJOR  := $(shell echo $(LATEST) | cut -d. -f1 | tr -d v)
MINOR  := $(shell echo $(LATEST) | cut -d. -f2)
PATCH  := $(shell echo $(LATEST) | cut -d. -f3)

NEXT_PATCH := v$(MAJOR).$(MINOR).$(shell expr $(PATCH) + 1)
NEXT_MINOR := v$(MAJOR).$(shell expr $(MINOR) + 1).0
NEXT_MAJOR := v$(shell expr $(MAJOR) + 1).0.0

.PHONY: release

release:
	@printf "\nCurrent version: $(LATEST)\n\n"
	@printf "  1) patch  ->  $(NEXT_PATCH)   (bug fixes)\n"
	@printf "  2) minor  ->  $(NEXT_MINOR)   (new features)\n"
	@printf "  3) major  ->  $(NEXT_MAJOR)   (breaking changes)\n\n"
	@printf "Pick a release type [1-3]: "; \
	read choice; \
	case "$$choice" in \
		1) tag="$(NEXT_PATCH)" ;; \
		2) tag="$(NEXT_MINOR)" ;; \
		3) tag="$(NEXT_MAJOR)" ;; \
		*) printf "Invalid choice '$$choice'\n"; exit 1 ;; \
	esac; \
	printf "\nTagging $$tag and pushing...\n"; \
	git tag "$$tag" && git push origin "$$tag" && printf "Released $$tag\n"
