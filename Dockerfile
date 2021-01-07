FROM 		golang:1.15

WORKDIR		/fromage
ADD		. /fromage
RUN		CGO_ENABLED=0 GOOS=linux go build  -ldflags '-extldflags "-static"' .

FROM 		alpine/git:v2.26.2
COPY --from=0	/fromage/fromage /usr/local/bin/
ENTRYPOINT 	["/usr/local/bin/fromage"]
