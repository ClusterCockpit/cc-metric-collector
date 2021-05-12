# Use central installation
CENTRAL_INSTALL = false

# Path to central installation (if CENTRAL_INSTALL=true)
LIKWID_BASE=/usr/local
# LIKWID version (should be same major version as central installation, 5.1.x)
LIKWID_VERSION = 5.1.0
# Target user for LIKWID's accessdaemon (if CENTRAL_INSTALL=false)
DAEMON_USER=root
# Target group for LIKWID's accessdaemon (if CENTRAL_INSTALL=false)
DAEMON_GROUP=root


#################################################
# No need to change anything below this line
#################################################
INSTALL_FOLDER = ./likwid
BUILD_FOLDER = ./likwid/build

ifneq ($(strip $(CENTRAL_INSTALL)),true)
LIKWID_BASE := $(shell pwd)/$(INSTALL_FOLDER)
DAEMON_BASE := $(LIKWID_BASE)
GROUPS_BASE := $(LIKWID_BASE)/groups
all: $(INSTALL_FOLDER)/liblikwid.a $(INSTALL_FOLDER)/likwid-accessD cleanup prepare_collector
else
DAEMON_BASE= $(LIKWID_BASE)/sbin
GROUPS_BASE := $(LIKWID_BASE)/share/likwid/perfgroups
all: $(INSTALL_FOLDER)/liblikwid.a cleanup prepare_collector
endif



$(BUILD_FOLDER)/likwid-$(LIKWID_VERSION).tar.gz: $(BUILD_FOLDER)
	wget -P $(BUILD_FOLDER) ftp://ftp.rrze.uni-erlangen.de/mirrors/likwid/likwid-$(LIKWID_VERSION).tar.gz

$(BUILD_FOLDER):
	mkdir -p $(BUILD_FOLDER)

$(INSTALL_FOLDER):
	mkdir -p $(INSTALL_FOLDER)

$(BUILD_FOLDER)/likwid-$(LIKWID_VERSION): $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION).tar.gz
	tar -C $(BUILD_FOLDER) -xf $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION).tar.gz

$(INSTALL_FOLDER)/liblikwid.a: $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION) $(INSTALL_FOLDER)
	sed -i -e s+"PREFIX ?= .*"+"PREFIX = $(LIKWID_BASE)"+g \
		-e s+"SHARED_LIBRARY = .*"+"SHARED_LIBRARY = false"+g \
		-e s+"INSTALLED_ACCESSDAEMON = .*"+"INSTALLED_ACCESSDAEMON = $(DAEMON_BASE)/likwid-accessD"+g \
		-e s+"LIKWIDGROUPPATH = .*"+"LIKWIDGROUPPATH = $(GROUPS_BASE)"+g \
		$(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/config.mk
	cd $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION) && make
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/liblikwid.a $(INSTALL_FOLDER)
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/ext/hwloc/liblikwid-hwloc.a $(INSTALL_FOLDER)
	cp -r $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/groups $(INSTALL_FOLDER)
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/src/includes/likwid*.h $(INSTALL_FOLDER)
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/src/includes/bstrlib.h $(INSTALL_FOLDER)

$(INSTALL_FOLDER)/likwid-accessD: $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/likwid-accessD
	sudo -u $(DAEMON_USER) -g $(DAEMON_GROUP) install -m 4775 $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/likwid-accessD $(INSTALL_FOLDER)

prepare_collector: likwidMetric.go
	cp likwidMetric.go likwidMetric.go.orig
	sed -i -e s+"const GROUPPATH =.*"+"const GROUPPATH = \`$(GROUPS_BASE)\`"+g likwidMetric.go

cleanup:
	rm -rf $(BUILD_FOLDER)
