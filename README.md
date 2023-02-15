# Name
  fromage - list, checks and bump container references in Dockerfiles in git repositories

# Usage

```
  fromage list  [--verbose] [--format=FORMAT] [--no-header] [--only-references]  [--branch=BRANCH ...] URL
  fromage check [--verbose] [--format=FORMAT] [--no-header] [--only-references]  [--branch=BRANCH ...] [--pin=LEVEL] URL
  fromage bump  [--verbose] [--dry-run] [--pin=LEVEL] [--latest] --branch=BRANCH URL
  fromage move  [--verbose] [--dry-run] --from=FROM_REPOSITORY --to=TO_REPOSITORY --branch=BRANCH URL
```

# Options
```
--branch=BRANCH        to inspect, defaults to all branches.
--format=FORMAT        to print: text, json or yaml [default: text].
--no-header            do not print header if output type is text.
--only-references      output only container image references.
--pin=LEVEL            pins the MAJOR or MINOR version level
--latest               bump to the latest version available
--from=FROM_REPOSITORY from repository context
--to=TO_REPOSITORY     to repository context

```

# Description
fromage lists all container references in dockerfiles in your git repository and indicates whether there are
newer versions available. To show all container images references in Dockerfiles, type:

```sh
./fromage list --branch master --verbose https://github.com/binxio/kritis
IMAGE                                   PATH                                            BRANCH  NEWER
golang:1.12                             helm-hooks/Dockerfile                           master  1.13,1.14,1.15
gcr.io/gcp-runtimes/ubuntu_16_0_4       helm-release/Dockerfile                         master  
ubuntu:trusty                           vendor/golang.org/x/net/http2/Dockerfile        master  
golang:1.12                             deploy/Dockerfile                               master  1.13,1.14,1.15
gcr.io/distroless/base:latest           deploy/Dockerfile                               master  
gcr.io/google-appengine/debian10:latest deploy/gcr-kritis-signer/Dockerfile             master  
gcr.io/gcp-runtimes/ubuntu_16_0_4       deploy/kritis-int-test/Dockerfile               master  
gcr.io/google-appengine/debian10:latest deploy/kritis-signer/Dockerfile                 master  
```

The columns show the container reference, the filename and branch in which it was found and available newer
versions.

## checking out-of-date references
to check whether there are newer references available, type:  
```sh
./fromage check --branch master --verbose https://github.com/binxio/kritis
IMAGE                                   PATH                                            BRANCH  NEWER
golang:1.12                             helm-hooks/Dockerfile                           master  1.13,1.14,1.15
golang:1.12                             deploy/Dockerfile                               master  1.13,1.14,1.15
exit code 1
```
This will only list the references which are out of date. If found, it exits with code 1.


## bumping container references
To bump the references to the next level, type:

```
./fromage bump --branch master --verbose git@github.com:binxio/kritis.git
2021/01/21 21:05:42 INFO: updating reference golang:1.12 to golang:1.13 in helm-hooks/Dockerfile
2021/01/21 21:05:42 INFO: updating reference golang:1.12 to golang:1.13 in helm-hooks/Dockerfile
2021/01/21 21:05:46 INFO: updating reference golang:1.12 to golang:1.13 in deploy/Dockerfile
2021/01/21 21:05:46 INFO: changes committed with 67847a0
2021/01/21 21:05:46 INFO: pushing changes to git@github.com:binxio/kritis.git
``` 

As you can see from the available versions, this process can be repeated until golang is at 
the highest level.

The bump will commit the changes to the repository. If it is a 
remote repository reference, the change will also be pushed.

Read more at [How to keep your Dockerfile container image references up-to-date](https://binx.io/blog/2021/01/30/how-to-keep-your-dockerfile-container-image-references-up-to-date/)

## moving container registry

If you need to move your container registry images from for instance docker hub to AWS Public ECR registry, type:

```
$ fromage move --verbose --from index.docker.io/library --to public.aws.ecr/docker/library --branch master git@github.com:binxio/kritis.git
2023/02/15 16:02:43 INFO: updating reference ubuntu:trusty to public.aws.ecr/docker/library/ubuntu:trusty in vendor/golang.org/x/net/http2/Dockerfile
2023/02/15 16:02:43 INFO: updating reference golang:1.13 to public.aws.ecr/docker/library/golang:1.13 in deploy/Dockerfile
2023/02/15 16:02:43 INFO: updating reference golang:1.13 to public.aws.ecr/docker/library/golang:1.13 in helm-hooks/Dockerfile
2023/02/15 16:02:43 INFO: updating reference golang:1.13 to public.aws.ecr/docker/library/golang:1.13 in helm-hooks/Dockerfile
2023/02/15 16:02:43 INFO: moved references from index.docker.io/library to public.aws.ecr/docker/library
2023/02/15 16:02:43 INFO: changes committed with 1234ef
2023/02/15 16:02:43 INFO: pushing changes to git@github.com:binxio/kritis.git
``` 

# Caveats
- The bump will update all container references it finds in all files
