
all: libganglia.so

libganglia.so:
	@find /usr ! -readable -prune -o -type d ! -executable -prune -o -name "$@*" -print0 | \
	    xargs --null --no-run-if-empty --replace \
	        ln --symbolic --verbose --force '{}' "$@"
	@if [[ ! -e "$@" ]]; then touch "$@"; fi


clean:
	rm -f libganglia.so

.PHONY: clean
