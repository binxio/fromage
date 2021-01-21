package main

import (
	"github.com/binxio/fromage/tag"
	"github.com/docopt/docopt-go"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"
)

type Fromage struct {
	List           bool
	Bump           bool
	Format         string
	OnlyReferences bool
	NoHeader       bool
	Branch         []string
	Url            string
	DryRun         bool
	Verbose        bool
	Pin            string
	repository     *git.Repository
	workTree       *git.Worktree
	currentBranch  *plumbing.Reference
	dockerfile     string
	references     DockerfileFromReferences
	pin            *tag.Level
	updated        bool
}

func (f *Fromage) IsLocalRepository() bool {
	return !MatchesScheme(f.Url) && !MatchesScpLike(f.Url)
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

func ReadFile(wt *git.Worktree, filename string) ([]byte, error) {
	file, err := wt.Filesystem.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func WriteFile(wt *git.Worktree, filename string, content []byte) error {
	file, err := wt.Filesystem.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(content)
	if err != nil {
		return err
	}

	_, err = wt.Add(filename)
	return err
}

func ReadFromStatements(wt *git.Worktree, filename string) ([]string, error) {
	content, err := ReadFile(wt, filename)
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

func (f *Fromage) OpenRepository() {
	var err error
	if f.Verbose {
		f.repository, err = Clone(f.Url, os.Stderr)
	} else {
		f.repository, err = Clone(f.Url, nil)
	}

	if err != nil {
		log.Printf("ERROR: failed to clone repository %s, %s", f.Url, err)
		os.Exit(1)
	}

	f.workTree, err = f.repository.Worktree()
	if err != nil {
		log.Printf("ERROR: failed to get repository worktree of %s, %s", f.Url, err)
		os.Exit(1)
	}

	f.references = make(DockerfileFromReferences, 0)

	for _, branch := range f.Branch {
		found := false
		_ = f.Branches().ForEach(func(reference *plumbing.Reference) error {
			found = found || (branch == reference.Name().Short() || branch == reference.Name().String())
			return nil
		})
		if !found {
			log.Printf("ERROR: branch %s does not exist", branch)
			os.Exit(1)
		}
	}
}

func (f Fromage) Branches() storer.ReferenceIter {
	branches, err := f.repository.Branches()
	if err != nil {
		log.Printf("failed retrieve branches of repository %s, %s", f.Url, err)
		os.Exit(1)
	}
	return branches
}

func (f *Fromage) ForEachDockerfile(m func(f *Fromage) error) error {
	return f.Branches().ForEach(func(ref *plumbing.Reference) error {
		f.currentBranch = ref

		if !DesiredBranch(ref, f.Branch) {
			return nil
		}

		dockerfiles, err := FindDockerfiles(f.workTree, "/")
		if err != nil {
			return err
		}
		for _, f.dockerfile = range dockerfiles {
			if err = m(f); err != nil {
				return err
			}
		}
		return nil
	})
}

func ListAllReferences(f *Fromage) error {
	references, err := ReadFromStatements(f.workTree, f.dockerfile)
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
			Branch:    f.currentBranch.Name().Short(),
			Path:      f.dockerfile,
			Reference: reference,
			Newer:     newer,
		}
		f.references = append(f.references, &froms)
	}
	return nil
}

func BumpReferences(f *Fromage) error {
	content, err := ReadFile(f.workTree, f.dockerfile)
	if err != nil {
		return err
	}


	content, updated := UpdateAllFromStatements(content, f.dockerfile, f.pin, true)
	if updated {
		f.updated = true
		if !f.DryRun {
			return WriteFile(f.workTree, f.dockerfile, content)
		}
	}

	return nil
}

func main() {
	usage := `fromage - list all container references in Dockerfiles in a git repository

Usage:
<<<<<<< HEAD
  fromage list [--verbose] [--format=FORMAT] [--no-header] [--only-references]  [--branch=BRANCH ...] URL
  fromage bump [--verbose] [--dry-run] [--pin=LEVEL] --branch=BRANCH URL
=======
  fromage list [--format=FORMAT] [--no-header] [--only-references]  [--branch=BRANCH ...] URL
  fromage bump [--dry-run] --branch=BRANCH URL
>>>>>>> 3fbe5730ac2a96ebdb8da4591aac7165458c6352

Options:
--branch=BRANCH     to inspect, defaults to all branches.
--format=FORMAT     to print: text, json or yaml [default: text].
--no-header         do not print header if output type is text.
--only-references   output only container image references.
--pin=LEVEL         pins the MAJOR or MINOR version level in the bump
`
	var fromage Fromage

	if opts, err := docopt.ParseDoc(usage); err == nil {
		if err = opts.Bind(&fromage); err != nil {
			log.Fatal(err)
		}
		if fromage.Pin != "" {
			if limit, err := tag.MakeLevelFromString(fromage.Pin); err != nil {
				log.Fatal(err)
			} else {
				fromage.pin = &limit
			}
		}
	} else {
		log.Fatal(err)
	}

	fromage.OpenRepository()

	if fromage.List {
		if err := fromage.ForEachDockerfile(ListAllReferences); err != nil {
			log.Fatal(err)
		}

		if fromage.OnlyReferences {
			fromage.references.OutputOnlyReferences(fromage.Format, fromage.NoHeader)
		} else {
			fromage.references.Output(fromage.Format, fromage.NoHeader)
		}
	}
	if fromage.Bump {
		if err := fromage.ForEachDockerfile(BumpReferences); err != nil {
			log.Fatal(err)
		}
		if err := fromage.CommitAndPush(); err != nil {
			log.Fatal(err)
		}
	}
}

func (f *Fromage) CommitAndPush() error {
	if !f.updated {
		return nil
	}
	if !f.DryRun {
		hash, err := f.workTree.Commit("you were fromaged", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "fromage",
				Email: "fromage@binx.io",
				When:  time.Now(),
			},
		})
		if err != nil {
			return err
		}
		log.Printf("INFO: changes committed with %s", hash.String()[0:7])
	} else {
		log.Printf("INFO: changes would be committed")
	}

	if f.IsLocalRepository() {
		return nil
	}

	if !f.DryRun {
		progress := os.Stderr
		if f.Verbose {
			progress = nil
		}
		log.Printf("INFO: pushing changes to %s", f.Url)
		return f.repository.Push(&git.PushOptions{Progress: progress})
	} else {
		log.Printf("INFO: changes would be pushed to %s", f.Url)
	}
	return nil
}
