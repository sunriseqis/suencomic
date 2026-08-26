.PHONY: all build-frontend build-backend build clean docker run

all: build

build-frontend:
	cd web && npm install && npm run build

build-backend:
	go build -ldflags="-s -w" -o suencomic .
	ln -sf suencomic manga-downloader

build: build-frontend build-backend

run: build
	./suencomic -port 8090

docker:
	docker compose up --build -d

clean:
	rm -rf suencomic manga-downloader web/dist ./download/.cache
