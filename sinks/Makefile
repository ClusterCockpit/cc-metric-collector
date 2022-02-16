
all: libganglia.so

libganglia.so:
	@find /usr -readable -name "libganglia.so*" -exec ln -sf {} libganglia.so \;


clean:
	rm -f libganglia.so

.PHONY: clean
