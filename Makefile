PREFIX=/usr/local
PROGRAM=vastai_exporter

.PHONY: build clean install uninstall

bin/$(PROGRAM): src/*.go
	@GOOS=linux GOARCH=amd64 go build -o bin/$(PROGRAM) src/*.go

build: bin/$(PROGRAM)

clean:
	@rm -rf ./bin

install: bin/$(PROGRAM) uninstall
	mkdir -p $(PREFIX)/bin
	cp bin/$(PROGRAM) bin/vast $(PREFIX)/bin/
	cd systemd && ./install

uninstall:
	systemctl stop $(PROGRAM) 2>/dev/null | true
	systemctl disable $(PROGRAM) 2>/dev/null | true
	rm -f $(PREFIX)/bin/$(PROGRAM) 2>/dev/null | true
