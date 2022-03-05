
all: likwid


# LIKWID version
LIKWID_VERSION = 5.2.1

.ONESHELL:
.PHONY: likwid
likwid:
	INSTALL_FOLDER="$${PWD}/likwid"
	BUILD_FOLDER="$${PWD}/likwidbuild"
	if [ -d $${INSTALL_FOLDER} ]; then rm -r $${INSTALL_FOLDER}; fi
	mkdir --parents --verbose  $${INSTALL_FOLDER} $${BUILD_FOLDER}
	wget -P "$${BUILD_FOLDER}" ftp://ftp.rrze.uni-erlangen.de/mirrors/likwid/likwid-$(LIKWID_VERSION).tar.gz
	tar -C $${BUILD_FOLDER} -xf $${BUILD_FOLDER}/likwid-$(LIKWID_VERSION).tar.gz
	install -Dpm 0644 $${BUILD_FOLDER}/likwid-$(LIKWID_VERSION)/src/includes/likwid*.h $${INSTALL_FOLDER}/
	install -Dpm 0644 $${BUILD_FOLDER}/likwid-$(LIKWID_VERSION)/src/includes/bstrlib.h $${INSTALL_FOLDER}/
	rm -r $${BUILD_FOLDER}


clean:
	rm -rf likwid

.PHONY: clean
