package main

import (
	"github.com/binxio/git-fromage/tag"
	"github.com/docopt/docopt-go"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"io/ioutil"
	"log"
	"os"
	"path"
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
  fromage list [--format=FORMAT] [--no-header] [--only-references]  [--branch=BRANCH ...] URL

Options:
--branch=BRANCH     to inspect, defaults to all branches.
--format=FORMAT     to print: text, json or yaml [default: text].
--no-header         do not print header if output type is text.
--only-references   output only container image references.

`
	var args struct {
		List           bool
		Update         bool
		Format         string
		OnlyReferences bool
		NoHeader       bool
		Branch         []string
		Url            string
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
	if args.Update {

	}

	if args.List {
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

					var newer []string
					if successors, err := tag.GetAllSuccessorsByString(reference); err == nil {
						newer = make([]string, 0, len(successors))
						for _, v := range successors {
							newer = append(newer, v.String())
						}
					}

					froms := DockerfileFromReference{
						Branch:    ref.Name().Short(),
						Path:      filename,
						Reference: reference,
						Newer:     newer,
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
}
