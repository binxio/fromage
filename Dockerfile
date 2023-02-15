FROM 		golang:1.19

WORKDIR		/fromage
ADD		. /fromage
RUN		CGO_ENABLED=0 GOOS=linux go build  -ldflags '-extldflags "-static"' .

FROM 		index.docker.io/alpine/git:v2.32.0
COPY --from=0	/fromage/fromage /usr/local/bin/
ENTRYPOINT 	["/usr/local/bin/fromage"]
