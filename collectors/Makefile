# LIKWID version
LIKWID_VERSION = 5.2.1
LIKWID_INSTALLED_FOLDER=$(shell dirname $(shell which likwid-topology 2>/dev/null) 2>/dev/null)

LIKWID_FOLDER="$(shell pwd)/likwid"

all: $(LIKWID_FOLDER)/likwid.h

.ONESHELL:
.PHONY: $(LIKWID_FOLDER)/likwid.h
$(LIKWID_FOLDER)/likwid.h:
	if [ "$(LIKWID_INSTALLED_FOLDER)" != "" ]; then \
		BASE="$(LIKWID_INSTALLED_FOLDER)/../include"; \
		mkdir -p $(LIKWID_FOLDER); \
		cp $$BASE/*.h $(LIKWID_FOLDER); \
	else \
		BUILD_FOLDER="$${PWD}/likwidbuild"; \
		if [ -d $(LIKWID_FOLDER) ]; then rm -r $(LIKWID_FOLDER); fi; \
		mkdir --parents --verbose  $(LIKWID_FOLDER) $${BUILD_FOLDER}; \
		wget -P "$${BUILD_FOLDER}" http://ftp.rrze.uni-erlangen.de/mirrors/likwid/likwid-$(LIKWID_VERSION).tar.gz; \
		tar -C $${BUILD_FOLDER} -xf $${BUILD_FOLDER}/likwid-$(LIKWID_VERSION).tar.gz; \
		install -Dpm 0644 $${BUILD_FOLDER}/likwid-$(LIKWID_VERSION)/src/includes/likwid*.h $(LIKWID_FOLDER)/; \
		install -Dpm 0644 $${BUILD_FOLDER}/likwid-$(LIKWID_VERSION)/src/includes/bstrlib.h $(LIKWID_FOLDER)/; \
		rm -r $${BUILD_FOLDER}; \
	fi


clean:
	rm -rf likwid

.PHONY: clean
