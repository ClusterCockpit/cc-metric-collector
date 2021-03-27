
LIKWID_VERSION = 5.1.0
DAEMON_USER=root
DAEMON_GROUP=root
INSTALL_FOLDER = ./likwid
BUILD_FOLDER = ./likwid/build

all: $(INSTALL_FOLDER)/liblikwid.a cleanup

$(BUILD_FOLDER)/likwid-$(LIKWID_VERSION).tar.gz: $(BUILD_FOLDER)
	wget -P $(BUILD_FOLDER) ftp://ftp.rrze.uni-erlangen.de/mirrors/likwid/likwid-$(LIKWID_VERSION).tar.gz

$(BUILD_FOLDER):
	mkdir -p $(BUILD_FOLDER)

$(BUILD_FOLDER)/likwid-$(LIKWID_VERSION): $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION).tar.gz
	tar -C $(BUILD_FOLDER) -xf $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION).tar.gz

$(INSTALL_FOLDER)/liblikwid.a: $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)
	sed -i -e s+"PREFIX ?= .*"+"PREFIX = $(shell pwd)/$(INSTALL_FOLDER)"+g \
		-e s+"SHARED_LIBRARY = .*"+"SHARED_LIBRARY = false"+g \
		-e s+"INSTALLED_ACCESSDAEMON = .*"+"INSTALLED_ACCESSDAEMON = $(shell pwd)/$(INSTALL_FOLDER)/likwid-accessD"+g \
		$(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/config.mk
	cd $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION) && make
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/liblikwid.a $(INSTALL_FOLDER)
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/ext/hwloc/liblikwid-hwloc.a $(INSTALL_FOLDER)
	cp -r $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/groups $(INSTALL_FOLDER)
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/src/includes/likwid*.h $(INSTALL_FOLDER)
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/src/includes/bstrlib.h $(INSTALL_FOLDER)
	sudo -u $(DAEMON_USER) -g $(DAEMON_GROUP) install -m 4775 $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/likwid-accessD $(INSTALL_FOLDER)

cleanup:
	rm -rf $(BUILD_FOLDER)
