# Name
  fromage - list and bump container references in Dockerfiles in git repositories

# Usage

```
  fromage list [--verbose] [--format=FORMAT] [--no-header] [--only-references]  [--branch=BRANCH ...] URL
  fromage bump [--verbose] [--dry-run] [--pin=LEVEL] --branch=BRANCH URL
```
# Options

```
  --branch=BRANCH     to update or to inspect default is all branches.
  --format=FORMAT     to print: text, json or yaml [default: text].
  --no-header         do not print header if output type is text.
  --only-references   output only container image references.
  --pin=LEVEL         pins the MAJOR or MINOR version level in the bump
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


To bump the references to the next file, type:

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

## bumping
Bumping a minor version, will bump to the next minor version.  Bumping a patch version, will always bump 
to the highest patch level available. If the next patch level is a next minor version, it will bump
the minor version unless `--pin` is specified. pin will only update within the major or minor version level.

# Caveats
- bumping algoritm may be subject to change
- The bump will update all container references it finds in all files (on the todo list)
