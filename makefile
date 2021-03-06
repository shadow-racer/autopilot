.PHONY = all clean tests deploy remote-pilot remote-pilot-dep

PLATFORM_ARM = GOARM=7 GOARCH=arm GOOS=linux

all: tests calibrate deploy

clean:
	rm bin/calibrate bin/selftest bin/unittest

tests: unittest

calibrate: cmd/calibrate/main.go
	cd cmd/calibrate && ${PLATFORM_ARM} go build -o ../../bin/calibrate main.go
	
unittest: test/unittest/main.go
	go build -o bin/unittest test/unittest/main.go

hacks:
	#${PLATFORM_ARM} go build -o bin/frameserver test/frameserver/main.go
	#scp -i ${PI_KEY} -r bin/frameserver cloud@${PI_TARGET}:/home/cloud/
	#${PLATFORM_ARM} go build -o bin/picam test/picam/main.go
	#scp -i ${PI_KEY} -r bin/picam cloud@${PI_TARGET}:/home/cloud/
	scp -i ${PI_KEY} -r pkg/parts/camera/camera.py cloud@${PI_TARGET}:/home/cloud/

remote-pilot: cmd/remote-pilot/main.go
	${PLATFORM_ARM} go build -o bin/remote-pilot cmd/remote-pilot/main.go
	scp -i ${PI_KEY} -r bin/remote-pilot cloud@${PI_TARGET}:/home/cloud/

remote-pilot-dep:
	scp -i ${PI_KEY} -r pkg/parts/scripts/camera.py cloud@${PI_TARGET}:/home/cloud/
	scp -i ${PI_KEY} -r cmd/remote-pilot/public cloud@${PI_TARGET}:/home/cloud/

deploy:
	scp -i ${PI_KEY} -r bin/calibrate cloud@${PI_TARGET}:/home/cloud/
	