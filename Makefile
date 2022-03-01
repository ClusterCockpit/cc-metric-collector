APP = cc-metric-collector
GOSRC_APP        := metric-collector.go
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


.PHONY: all
all: $(APP)

$(APP): $(GOSRC)
	make -C collectors
	go get
	go build -o $(APP) $(GOSRC_APP)

.PHONY: clean
.ONESHELL:
clean:
	@for COMP in $(COMPONENT_DIRS)
	do
	    if [[ -e $$COMP/Makefile ]]; then
	        make -C $$COMP clean
	    fi
	done
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
	@VERS=$$(git describe --tags)
	@VERS=$${VERS#v}
	@VERS=$${VERS//-/_}
	@PREFIX=$$(rpmspec --query --queryformat "%{name}-%{version}" --define="VERS $${VERS}" "$${SPECFILE}")
	@FORMAT="tar.gz"
	@SRCFILE="$${SOURCEDIR}/$${PREFIX}.$${FORMAT}"
	@git archive --verbose --format "$${FORMAT}" --prefix="$${PREFIX}/" --output="$${SRCFILE}" HEAD
	# Build RPM and SRPM
	@rpmbuild -ba --define="VERS $${VERS}" --rmsource --clean "$${SPECFILE}"
	# Report RPMs and SRPMs when in GitHub Workflow
	@if [[ "$${GITHUB_ACTIONS}" == true ]]; then
	@     RPMFILES="$${RPMDIR}"/*/*.rpm
	@     SRPMFILES="$${SRPMDIR}"/*.src.rpm
	@     echo "RPMs: $${RPMFILES}"
	@     echo "SRPMs: $${SRPMFILES}"
	@     echo "::set-output name=SRPM::$${SRPMFILES}"
	@     echo "::set-output name=RPM::$${RPMFILES}"
	@fi
