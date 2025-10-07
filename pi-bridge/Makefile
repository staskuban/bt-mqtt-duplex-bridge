.PHONY: build deploy clean

build:
	GOOS=linux GOARCH=arm64 go build -o elm327-bridge main.go

deploy: build
	scp -i ~/.ssh/id_rsa elm327-bridge user@192.168.28.190:/home/user/elm327/

clean:
	rm -f elm327-bridge