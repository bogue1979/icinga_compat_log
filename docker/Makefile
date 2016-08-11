VERSION ?= 0.2.2
DOCKERACCOUNT ?= bogue1979

.DEFAULT_GOAL := build

build: nats-streaming-server
	docker build -t $(DOCKERACCOUNT)/nats-streaming-server . && touch build

package_$(VERSION).zip:
	wget -O package_$(VERSION).zip https://github.com/nats-io/nats-streaming-server/releases/download/v$(VERSION)/nats-streaming-server-linux-amd64.zip

nats-streaming-server: package_$(VERSION).zip
	unzip package_$(VERSION).zip
	cp nats-streaming-server-linux-amd64/nats-streaming-server .
	rm -rf nats-streaming-server-linux-amd64


stop:
	docker stop nats-streaming-server || exit 0

rm: stop
	docker rm nats-streaming-server || exit 0

run: build rm
	docker run -d --name nats-streaming-server -p 4222:4222 $(DOCKERACCOUNT)/nats-streaming-server ./nats-streaming-server --user icinga --pass password

clean: rm
	rm -rf nats-streaming-server  package_$(VERSION).zip build || exit 0
