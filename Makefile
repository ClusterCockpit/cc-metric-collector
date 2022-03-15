APP = cc-metric-collector
GOSRC_APP        := cc-metric-collector.go
GOSRC_COLLECTORS := $(wildcard collectors/*.go)
GOSRC_SINKS      := $(wildcard sinks/*.go)
GOSRC_RECEIVERS  := $(wildcard receivers/*.go)
GOSRC_INTERNAL   := $(wildcard internal/*/*.go)
GOSRC            := $(GOSRC_APP) $(GOSRC_COLLECTORS) $(GOSRC_SINKS) $(GOSRC_RECEIVERS) $(GOSRC_INTERNAL)
COMPONENT_DIRS   := collectors \
			sinks \
			receivers \
			internal/metricRouter \
			internal/ccMetric \
			internal/metricAggregator \
			internal/ccLogger \
			internal/ccTopology \
			internal/multiChanTicker

BINDIR = bin


.PHONY: all
all: $(APP)

$(APP): $(GOSRC)
	make -C collectors
	go get
	go build -o $(APP) $(GOSRC_APP)

install: $(APP)
	@WORKSPACE=$(PREFIX)
	@if [ -z "$${WORKSPACE}" ]; then exit 1; fi
	@mkdir --parents --verbose $${WORKSPACE}/usr/$(BINDIR)
	@install -Dpm 755 $(APP) $${WORKSPACE}/usr/$(BINDIR)/$(APP)
	@mkdir --parents --verbose $${WORKSPACE}/etc/cc-metric-collector $${WORKSPACE}/etc/default $${WORKSPACE}/etc/systemd/system $${WORKSPACE}/etc/init.d
	@install -Dpm 600 config.json $${WORKSPACE}/etc/cc-metric-collector/cc-metric-collector.json
	@sed -i -e s+"\"./"+"\"/etc/cc-metric-collector/"+g $${WORKSPACE}/etc/cc-metric-collector/cc-metric-collector.json
	@install -Dpm 600 sinks.json $${WORKSPACE}/etc/cc-metric-collector/sinks.json
	@install -Dpm 600 collectors.json $${WORKSPACE}/etc/cc-metric-collector/collectors.json
	@install -Dpm 600 router.json $${WORKSPACE}/etc/cc-metric-collector/router.json
	@install -Dpm 600 receivers.json $${WORKSPACE}/etc/cc-metric-collector/receivers.json
	@install -Dpm 600 scripts/cc-metric-collector.config $${WORKSPACE}/etc/default/cc-metric-collector
	@install -Dpm 644 scripts/cc-metric-collector.service $${WORKSPACE}/etc/systemd/system/cc-metric-collector.service
	@install -Dpm 644 scripts/cc-metric-collector.init $${WORKSPACE}/etc/init.d/cc-metric-collector


.PHONY: clean
.ONESHELL:
clean:
	@for COMP in $(COMPONENT_DIRS); do if [ -e $$COMP/Makefile ]; then make -C $$COMP clean;  fi; done
	rm -f $(APP)

.PHONY: fmt
fmt:
	go fmt $(GOSRC_COLLECTORS)
	go fmt $(GOSRC_SINKS)
	go fmt $(GOSRC_RECEIVERS)
	go fmt $(GOSRC_APP)
	@for F in $(GOSRC_INTERNAL); do go fmt $$F; done


# Examine Go source code and reports suspicious constructs
.PHONY: vet
vet:
	go vet ./...


# Run linter for the Go programming language.
# Using static analysis, it finds bugs and performance issues, offers simplifications, and enforces style rules
.PHONY: staticcheck
staticcheck:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	$$(go env GOPATH)/bin/staticcheck ./...

.ONESHELL:
.PHONY: RPM
RPM: scripts/cc-metric-collector.spec
	@WORKSPACE="$${PWD}"
	@SPECFILE="$${WORKSPACE}/scripts/cc-metric-collector.spec"
	# Setup RPM build tree
	@eval $$(rpm --eval "ARCH='%{_arch}' RPMDIR='%{_rpmdir}' SOURCEDIR='%{_sourcedir}' SPECDIR='%{_specdir}' SRPMDIR='%{_srcrpmdir}' BUILDDIR='%{_builddir}'")
	@mkdir --parents --verbose "$${RPMDIR}" "$${SOURCEDIR}" "$${SPECDIR}" "$${SRPMDIR}" "$${BUILDDIR}"
	# Create source tarball
	@COMMITISH="HEAD"
	@VERS=$$(git describe --tags $${COMMITISH})
	@VERS=$${VERS#v}
	@VERS=$$(echo $$VERS | sed -e s+'-'+'_'+g)
	@eval $$(rpmspec --query --queryformat "NAME='%{name}' VERSION='%{version}' RELEASE='%{release}' NVR='%{NVR}' NVRA='%{NVRA}'" --define="VERS $${VERS}" "$${SPECFILE}")
	@PREFIX="$${NAME}-$${VERSION}"
	@FORMAT="tar.gz"
	@SRCFILE="$${SOURCEDIR}/$${PREFIX}.$${FORMAT}"
	@git archive --verbose --format "$${FORMAT}" --prefix="$${PREFIX}/" --output="$${SRCFILE}" $${COMMITISH}
	# Build RPM and SRPM
	@rpmbuild -ba --define="VERS $${VERS}" --rmsource --clean "$${SPECFILE}"
	# Report RPMs and SRPMs when in GitHub Workflow
	@if [[ "$${GITHUB_ACTIONS}" == true ]]; then
	@     RPMFILE="$${RPMDIR}/$${ARCH}/$${NVRA}.rpm"
	@     SRPMFILE="$${SRPMDIR}/$${NVR}.src.rpm"
	@     echo "RPM: $${RPMFILE}"
	@     echo "SRPM: $${SRPMFILE}"
	@     echo "::set-output name=SRPM::$${SRPMFILE}"
	@     echo "::set-output name=RPM::$${RPMFILE}"
	@fi

.PHONY: DEB
DEB: scripts/cc-metric-collector.deb.control $(APP)
	@BASEDIR=$${PWD}
	@WORKSPACE=$${PWD}/.dpkgbuild
	@DEBIANDIR=$${WORKSPACE}/debian
	@DEBIANBINDIR=$${WORKSPACE}/DEBIAN
	@mkdir --parents --verbose $$WORKSPACE $$DEBIANBINDIR
	#@mkdir --parents --verbose $$DEBIANDIR
	@CONTROLFILE="$${BASEDIR}/scripts/cc-metric-collector.deb.control"
	@COMMITISH="HEAD"
	@VERS=$$(git describe --tags --abbrev=0 $${COMMITISH})
	@VERS=$${VERS#v}
	@VERS=$$(echo $$VERS | sed -e s+'-'+'_'+g)
	@ARCH=$$(uname -m)
	@ARCH=$$(echo $$ARCH | sed -e s+'_'+'-'+g)
	@PREFIX="$${NAME}-$${VERSION}_$${ARCH}"
	@SIZE_BYTES=$$(du -bcs --exclude=.dpkgbuild "$$WORKSPACE"/ | awk '{print $$1}' | head -1 | sed -e 's/^0\+//')
	@SIZE="$$(awk -v size="$$SIZE_BYTES" 'BEGIN {print (size/1024)+1}' | awk '{print int($$0)}')"
	#@sed -e s+"{VERSION}"+"$$VERS"+g -e s+"{INSTALLED_SIZE}"+"$$SIZE"+g -e s+"{ARCH}"+"$$ARCH"+g $$CONTROLFILE > $${DEBIANDIR}/control
	@sed -e s+"{VERSION}"+"$$VERS"+g -e s+"{INSTALLED_SIZE}"+"$$SIZE"+g -e s+"{ARCH}"+"$$ARCH"+g $$CONTROLFILE > $${DEBIANBINDIR}/control
	@make PREFIX=$${WORKSPACE} install
	@DEB_FILE="cc-metric-collector_$${VERS}_$${ARCH}.deb"
	@dpkg-deb -b $${WORKSPACE} "$$DEB_FILE"
	@rm -r "$${WORKSPACE}"
