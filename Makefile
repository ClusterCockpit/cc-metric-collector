APP = cc-metric-collector

all: $(APP)

$(APP): metric-collector.go
	make -C collectors
	go get
	go build -o $(APP) metric-collector.go

clean:
	make -C collectors clean
	rm -f $(APP)

fmt:
	go fmt collectors/*.go
	go fmt sinks/*.go
	go fmt receivers/*.go
	go fmt metric-collector.go

.PHONY: clean
