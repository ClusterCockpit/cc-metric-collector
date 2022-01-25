APP = cc-metric-collector
GOSRC_APP        := metric-collector.go
GOSRC_COLLECTORS := $(wildcard collectors/*.go)
GOSRC_SINKS      := $(wildcard sinks/*.go)
GOSRC_RECEIVERS  := $(wildcard receivers/*.go)
GOSRC_INTERNAL   := $(wildcard internal/*/*.go)
GOSRC            := $(GOSRC_APP) $(GOSRC_COLLECTORS) $(GOSRC_SINKS) $(GOSRC_RECEIVERS) $(GOSRC_INTERNAL)


.PHONY: all
all: $(APP)

$(APP): $(GOSRC)
	make -C collectors
	go get
	go build -o $(APP) $(GOSRC_APP)

.PHONY: clean
clean:
	make -C collectors clean
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
