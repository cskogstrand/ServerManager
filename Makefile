VERSION ?= dev-build

.PHONY: all build buildwin run rundebug rundebug-docker rundebug-docker-cold stopdebug-docker deps package clean webapp webapp-dev types test

all: deps clean buildwin build

webapp:
	cd webapp && (test -d node_modules || npm install) && npm run build

webapp-dev:
	cd webapp && (test -d node_modules || npm install) && npm run dev

types:
	tygo generate

test:
	cd webapp && npm test
	cd src && go vet .

build: webapp
	mkdir -p bin/
	go-assets-builder schema.sql favicon.ico ini webapp/dist -o src/assets.go
	cd src; CGO_ENABLED=1 go build -o ../bin/sm_linux -ldflags="-w -s -X 'main.Version=$(VERSION)'" .

buildwin: webapp
	mkdir -p bin/
	go-assets-builder schema.sql favicon.ico ini webapp/dist -o src/assets.go
	go-winres make
	cd src; CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 go build -o ../bin/sm_win.exe -ldflags="-w -s -X 'main.Version=$(VERSION)'" .

run: webapp
	go-assets-builder schema.sql favicon.ico ini webapp/dist -o src/assets.go
	cd src; go run .

rundebug:
	test -d webapp/dist || $(MAKE) webapp
	cd src; go run . -debug

rundebug-docker:
	docker compose -f docker-compose.dev.yml up --build

rundebug-docker-cold:
	docker compose -f docker-compose.dev.yml down -v
	docker compose -f docker-compose.dev.yml up --build

stopdebug-docker:
	docker compose -f docker-compose.dev.yml down

deps:
	go install github.com/jessevdk/go-assets-builder@latest
	go install github.com/tc-hib/go-winres@latest
	go install github.com/gzuidhof/tygo@latest
	go get main/src
	cd webapp && npm install

package: clean buildwin build

clean:
	rm -rf bin/
	rm -rf webapp/dist
	rm -f src/assets.go
	rm -f *.syso
	go clean
