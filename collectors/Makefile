# Use central installation
CENTRAL_INSTALL = false
# How to access hardware performance counters through LIKWID.
# Recommended is 'direct' mode
ACCESSMODE = direct

#######################################################################
# if CENTRAL_INSTALL == true
#######################################################################
# Path to central installation (if CENTRAL_INSTALL=true)
LIKWID_BASE=/apps/likwid/5.2.1
# LIKWID version (should be same major version as central installation, 5.2.x)
LIKWID_VERSION = 5.2.1

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
	cd "$(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)" && make "PREFIX=$(LIKWID_BASE)" "SHARED_LIBRARY=false" "ACCESSMODE=$(ACCESSMODE)" "INSTALLED_ACCESSDAEMON=$(DAEMON_INSTALLDIR)/likwid-accessD"
	cp \
	    $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/liblikwid.a \
	    $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/ext/hwloc/liblikwid-hwloc.a \
	    $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/src/includes/likwid*.h \
	    $(BUILD_FOLDER)/likwid-$(LIKWID_VERSION)/src/includes/bstrlib.h \
	    $(INSTALL_FOLDER)

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
