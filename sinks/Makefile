
all: libganglia.so

libganglia.so:
	@find /usr -readable -name "libganglia.so*" -print0 | \
	    xargs --null --no-run-if-empty --replace \
	        ln --symbolic --verbose --force '{}' libganglia.so


clean:
	rm -f libganglia.so

.PHONY: clean
