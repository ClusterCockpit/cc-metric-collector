APP = cc-metric-collector
GOSRC_APP        := metric-collector.go
GOSRC_COLLECTORS := $(wildcard collectors/*.go)
GOSRC_SINKS      := $(wildcard sinks/*.go)
GOSRC_RECEIVERS  := $(wildcard receivers/*.go)
GOSRC            := $(GOSRC_APP) $(GOSRC_COLLECTORS) $(GOSRC_SINKS) $(GOSRC_RECEIVERS)

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
	find . -name "*.go" -exec go fmt {} \;

