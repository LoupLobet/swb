CC=/usr/local/go/bin/go
all: swb

.PHONY: swb
swb:
	${CC} build -buildvcs=false

install: swb
	cp swb /usr/local/bin/swb 
