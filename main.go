package main

import (
	"github.com/docopt/docopt-go"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
)

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

func ExtractFromStatements(content []byte) []string {
	result := make([]string, 0)
	aliases := make(map[string]string, 0)
	references := make(map[string]bool, 0)

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
	return result
}

func ReadFromStatements(wt *git.Worktree, filename string) ([]string, error) {
	file, err := wt.Filesystem.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return ExtractFromStatements(content), nil
}

func DesiredBranch(reference *plumbing.Reference, branches []string) bool {
	if !reference.Name().IsBranch() {
		return false
	}
	for _, branch := range branches {
		if branch == reference.Name().Short() || branch == reference.Name().String() {
			return true
		}
	}
	return len(branches) == 0
}

func main() {
	usage := `fromage - list all container references in Dockerfiles in a git repository

Usage:
  fromage list [--help] [--format=FORMAT] [--no-header] [--only-references]  [--branch=BRANCH ...] URL

Options:
--branch=BRANCH     to inspect, defaults to all branches.
--format=FORMAT     to print: text, json or yaml [default: text].
--no-header         do not print header if output type is text.
--only-references   output only container image references.

`
	var args struct {
		List           bool
		Format         string
		OnlyReferences bool
		NoHeader       bool
		Branch         []string
		Url            string
		Help           bool
	}

	if opts, err := docopt.ParseDoc(usage); err == nil {
		if err = opts.Bind(&args); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal(err)
	}

	r, err := Clone(args.Url)
	if err != nil {
		log.Printf("failed to clone repository %s, %s", args.Url, err)
		os.Exit(1)
	}

	wt, err := r.Worktree()
	if err != nil {
		log.Printf("failed to get repository worktree of %s, %s", args.Url, err)
		os.Exit(1)
	}
	branches, err := r.Branches()
	if err != nil {
		log.Printf("failed retrieve branches of repository %s, %s", args.Url, err)
		os.Exit(1)
	}

	var result = make(DockerfileFromReferences, 0)

	err = branches.ForEach(func(ref *plumbing.Reference) error {
		if !DesiredBranch(ref, args.Branch) {
			return nil
		}

		dockerfiles, err := FindDockerfiles(wt, "/")
		if err != nil {
			return err
		}
		for _, filename := range dockerfiles {
			references, err := ReadFromStatements(wt, filename)
			if err != nil {
				return err
			}
			for _, reference := range references {

				froms := DockerfileFromReference{
					Branch:    ref.Name().Short(),
					Path:      filename,
					Reference: reference,
				}
				result = append(result, &froms)
			}
		}
		return nil
	},
	)
	if args.OnlyReferences {
		result.OutputOnlyReferences(args.Format, args.NoHeader)
	} else {
		result.Output(args.Format, args.NoHeader)
	}
}
