# LIKWID version
LIKWID_VERSION := 5.4.1
LIKWID_INSTALLED_FOLDER := $(shell dirname $$(which likwid-topology 2>/dev/null) 2>/dev/null)

LIKWID_FOLDER := $(CURDIR)/likwid

all: likwid

.ONESHELL:
.PHONY: likwid
likwid:
	if [ -n "$(LIKWID_INSTALLED_FOLDER)" ]; then
	    # Using likwid include files from system installation
	    INCLUDE_DIR="$(LIKWID_INSTALLED_FOLDER)/../include"
	    mkdir --parents --verbose "$(LIKWID_FOLDER)"
	    cp "$${INCLUDE_DIR}"/*.h "$(LIKWID_FOLDER)"
	else
	    # Using likwid include files from downloaded tar archive
	    if [ -d "$(LIKWID_FOLDER)" ]; then
	        rm --recursive "$(LIKWID_FOLDER)"
	    fi
	    BUILD_FOLDER="$${PWD}/likwidbuild"
	    mkdir --parents --verbose  "$${BUILD_FOLDER}"
	    wget --output-document=- http://ftp.rrze.uni-erlangen.de/mirrors/likwid/likwid-$(LIKWID_VERSION).tar.gz |
	        tar --directory="$${BUILD_FOLDER}" --extract --gz
		install -D --verbose --preserve-timestamps --mode=0644 --target-directory="$(LIKWID_FOLDER)" "$${BUILD_FOLDER}/likwid-$(LIKWID_VERSION)/src/includes"/likwid*.h
	    rm --recursive "$${BUILD_FOLDER}"
	fi


.PHONY: clean
clean:
	rm -rf likwid
