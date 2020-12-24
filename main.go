package main

import (
	"encoding/json"
	"fmt"
	"github.com/docopt/docopt-go"
	"golang.org/x/crypto/ssh"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	go_git_ssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
)

type DockerfileFromReferences struct {
	Url        string
	Branch     string
	Path       string
	References []string
}

func FindDockerfiles(wt *git.Worktree, filename string) ([]string, error) {
	result := make([]string, 0)
	file, err := wt.Filesystem.Stat(filename)
	if err != nil {
		return nil, err
	}
	if file.IsDir() {
		dir, err := wt.Filesystem.ReadDir(filename)
		if err != nil {
			return nil, err
		}

		for _, file = range dir {
			fullPath := path.Join(filename, file.Name())
			if filename == "/" {
				fullPath = file.Name()
			}
			found, err := FindDockerfiles(wt, fullPath)
			if err == nil {
				result = append(result, found...)
			} else {
				return nil, err
			}
		}
	} else {
		if path.Base(file.Name()) == "Dockerfile" {
			result = append(result, filename)
		}
	}
	return result, nil
}

var (
	fromRegExp      = regexp.MustCompile(`(?m)^\s*[Ff][Rr][Oo][Mm]\s+(?P<reference>[^\s]+)(\s*[Aa][Ss]\s+(?P<alias>[^\s]+))?.*$`)
	fromRegExpNames = fromRegExp.SubexpNames()
)

func ReadFromStatements(wt *git.Worktree, filename string) ([]string, error) {
	result := make([]string, 0)
	aliases := make(map[string]string, 0)
	references := make(map[string]bool, 0)

	file, err := wt.Filesystem.Open(filename)
	if err != nil {
		return nil, err
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	matches := fromRegExp.FindAllSubmatch(content, -1)
	if matches != nil {
		for _, match := range matches {
			alias := ""
			reference := ""
			for i, name := range fromRegExpNames {
				switch name {
				case "reference":
					reference = string(match[i])
				case "alias":
					alias = string(match[i])
				default:
					// ignore
				}
			}
			if alias != "" {
				// register the reference as an alias
				aliases[alias] = reference
			}
			if _, ok := aliases[reference]; !ok {
				// the reference is not pointing to an alias
				if _, ok := references[reference]; !ok {
					// the reference was not yet registered
					references[reference] = true
					result = append(result, reference)
				}
			}
		}
	}
	return result, nil
}

func main() {
	usage := `fromage - list all container references in Dockerfiles in a git repository

Usage:
  fromage [--help] [--identity=KEYFILE] [--branch=BRANCH ...] [URL] ...

Options:
--branch=BRANCH     to inspect, default is all.
--identity=KEYFILE  private key to authenticate with [default: $HOME/.ssh/id_rsa].
`
	var args struct {
		Identity string
		Branch []string
		Url []string
		Help bool
	}

	if opts, err := docopt.ParseDoc(usage); err == nil {
		if err = opts.Bind(&args); err != nil {
			log.Fatal(err)
		}
		if args.Identity == "" {
			args.Identity = "$HOME/.ssh/id_rsa"
		}
	} else {
		log.Fatal(err)
	}


	keyFile := os.ExpandEnv(args.Identity)

	sshKey, err := ioutil.ReadFile(keyFile)
	if err != nil {
		log.Printf("failed to read key file '%s', %s", keyFile, err)
		os.Exit(1)
	}

	signer, err := ssh.ParsePrivateKey(sshKey)
	if err != nil {
		log.Printf("failed to read private key from '%s', %s", keyFile, err)
		os.Exit(1)
	}

	for _, url := range args.Url {
		auth := &go_git_ssh.PublicKeys{User: "git", Signer: signer}
		r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
			URL:  url,
			Auth: auth,
		})
		if err != nil {
			log.Fatal(err)
		}

		err = r.Fetch(&git.FetchOptions{
			RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
			Depth:    1,
			Auth:     auth,
		})
		if err != nil {
			log.Fatal(err)
		}

		wt, err := r.Worktree()
		if err != nil {
			log.Fatal(err)
		}

		i, err := r.Branches()
		if err != nil {
			log.Fatal(err)
		}

		err = i.ForEach(func(ref *plumbing.Reference) error {
			if ref.Name().Short() == "HEAD" {
				return nil
			}

			fmt.Printf("%s\n", ref.Name().String())
			dockerfiles, err := FindDockerfiles(wt, "/")
			if err != nil {
				return err
			}
			for _, filename := range dockerfiles {
				references, err := ReadFromStatements(wt, filename)
				if err != nil {
					return err
				}
				froms := DockerfileFromReferences{
					Url: url,
					Branch:     ref.Name().Short(),
					Path:       filename,
					References: references,
				}
				encoder := json.NewEncoder(os.Stdout)
				_ = encoder.Encode(froms)
			}

			return nil
		},
		)
	}
}
