# Use central installation
CENTRAL_INSTALL = false
# How to access hardware performance counters through LIKWID.
# Recommended is 'direct' mode
ACCESSMODE = direct

#######################################################################
# if CENTRAL_INSTALL == true
#######################################################################
# Path to central installation (if CENTRAL_INSTALL=true)
LIKWID_BASE=/apps/likwid/5.2.0
# LIKWID version (should be same major version as central installation, 5.1.x)
LIKWID_VERSION = 5.2.0

#######################################################################
# if CENTRAL_INSTALL == false and ACCESSMODE == accessdaemon
#######################################################################
# Where to install the accessdaemon
DAEMON_INSTALLDIR = /usr/local
# Which user to use for the accessdaemon
DAEMON_USER = root
# Which group to use for the accessdaemon
DAEMON_GROUP = root



#################################################
# No need to change anything below this line
#################################################
INSTALL_FOLDER = ./likwid
BUILD_FOLDER = ./likwid/build

ifneq ($(strip $(CENTRAL_INSTALL)),true)
LIKWID_BASE := $(shell pwd)/$(INSTALL_FOLDER)
DAEMON_BASE := $(LIKWID_BASE)
GROUPS_BASE := $(LIKWID_BASE)/groups
all: $(INSTALL_FOLDER)/liblikwid.a cleanup
else
DAEMON_BASE= $(LIKWID_BASE)/sbin
all: $(INSTALL_FOLDER)/liblikwid.a cleanup
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
		-e s+"ACCESSMODE = .*"+"ACCESSMODE = $(ACCESSMODE)"+g \
		-e s+"INSTALLED_ACCESSDAEMON = .*"+"INSTALLED_ACCESSDAEMON = $(DAEMON_INSTALLDIR)/likwid-accessD"+g \
		$(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/config.mk
	cd $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION) && make
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/liblikwid.a $(INSTALL_FOLDER)
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/ext/hwloc/liblikwid-hwloc.a $(INSTALL_FOLDER)
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/src/includes/likwid*.h $(INSTALL_FOLDER)
	cp $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/src/includes/bstrlib.h $(INSTALL_FOLDER)

$(DAEMON_INSTALLDIR)/likwid-accessD: $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/likwid-accessD
	sudo -u $(DAEMON_USER) -g $(DAEMON_GROUP) install -m 4775 $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/likwid-accessD $(DAEMON_INSTALLDIR)/likwid-accessD

prepare_collector: likwidMetric.go
	cp likwidMetric.go likwidMetric.go.orig
	sed -i -e s+"const GROUPPATH =.*"+"const GROUPPATH = \`$(GROUPS_BASE)\`"+g likwidMetric.go

cleanup:
	rm -rf $(BUILD_FOLDER)

clean: cleanup
	rm -rf likwid

.PHONY: clean
