package main

import (
	"bytes"
	"fmt"
	"github.com/binxio/fromage/tag"
	"github.com/docopt/docopt-go"
	"github.com/google/go-containerregistry/pkg/name"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

type Fromage struct {
	Check          bool
	List           bool
	Bump           bool
	Move           bool
	Format         string
	OnlyReferences bool
	NoHeader       bool
	Branch         []string
	Url            string
	DryRun         bool
	Verbose        bool
	Pin            string
	Latest         bool
	From, To       string

	repository    *git.Repository
	workTree      *git.Worktree
	currentBranch *plumbing.Reference
	dockerfile    string
	references    DockerfileFromReferences
	pin           *tag.Level
	updated       bool
}

func (f *Fromage) IsLocalRepository() bool {
	return !MatchesScheme(f.Url) && !MatchesScpLike(f.Url)
}

func FindDockerfiles(wt *git.Worktree, filename string, ref *plumbing.Reference) ([]string, error) {
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
			found, err := FindDockerfiles(wt, fullPath, ref)
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

func (f *Fromage) ReadOnly() bool {
	return f.Check || f.List || f.DryRun
}

func (f *Fromage) OpenRepository() {
	var err error

	if f.Verbose {
		f.repository, err = Clone(f.Url, os.Stderr, f.ReadOnly())
	} else {
		f.repository, err = Clone(f.Url, &bytes.Buffer{}, f.ReadOnly())
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

		if f.Verbose {
			log.Printf("checking out %s\n", ref.Name().Short())
		}

		err := f.workTree.Checkout(&git.CheckoutOptions{
			Branch: ref.Name(),
			Force:  false,
		})
		if err != nil {
			return fmt.Errorf("ERROR: checkout of %s failed, %s", ref.Name().Short(), err)
		}

		dockerfiles, err := FindDockerfiles(f.workTree, "/", ref)
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
		if successors, err := tag.GetAllSuccessorsByString(reference, f.pin); err == nil {
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

	content, updated := UpdateAllFromStatements(content, f.dockerfile, f.pin, f.Latest, f.Verbose)
	if updated {
		f.updated = true
		if !f.DryRun {
			return WriteFile(f.workTree, f.dockerfile, content)
		}
	}

	return nil
}

func moveImageReferences(content []byte, filename string, verbose bool, from, to string) ([]byte, bool, error) {
	updated := false
	refs := ExtractFromStatements(content)
	for _, refString := range refs {
		ref, err := name.ParseReference(refString)
		fullRef := ref.Name()

		if err != nil {
			return nil, false, err
		}

		if !strings.HasPrefix(fullRef, from) && len(fullRef) > len(from) {
			continue
		}

		if delimiter := fullRef[len(from)]; delimiter != ':' && delimiter != '/' && delimiter != '@' {
			continue
		}

		newRefString := to + fullRef[len(from):]
		if to == "index.docker.io/library" {
			newRefString = fullRef[len(from)+1:]
		}
		newRef, err := name.ParseReference(newRefString)
		if err != nil {
			return nil, false, err
		}

		if !RepositoryExists(newRef, verbose) {
			return nil, false, fmt.Errorf("ERROR: %s is not a valid image reference", newRef)
		}

		ok := false
		if content, ok = UpdateFromStatements(content, ref, newRef, filename, verbose); ok {
			updated = true
		}
	}
	return content, updated, nil
}

func MoveImageReferences(f *Fromage) error {
	content, err := ReadFile(f.workTree, f.dockerfile)
	if err != nil {
		return err
	}

	content, f.updated, err = moveImageReferences(content, f.dockerfile, f.Verbose, f.From, f.To)
	if err != nil {
		log.Fatalf("%s", err)
	}

	if f.updated {
		if !f.DryRun {
			return WriteFile(f.workTree, f.dockerfile, content)
		}
	}

	return nil
}

func main() {
	usage := `fromage - checks, list and bumps all container references in Dockerfiles in a git repository

Usage:
  fromage list  [--verbose] [--format=FORMAT] [--no-header] [--only-references]  [--branch=BRANCH ...] URL
  fromage check [--verbose] [--format=FORMAT] [--no-header] [--only-references]  [--branch=BRANCH ...] [--pin=LEVEL] URL
  fromage bump  [--verbose] [--dry-run] [--pin=LEVEL] [--latest] --branch=BRANCH URL
  fromage move  [--verbose] [--dry-run] --from=FROM_REPOSITORY --to=TO_REPOSITORY --branch=BRANCH URL

Options:
--branch=BRANCH        to inspect, defaults to all branches.
--format=FORMAT        to print: text, json or yaml [default: text].
--no-header            do not print header if output type is text.
--only-references      output only container image references.
--pin=LEVEL            pins the MAJOR or MINOR version level
--latest               bump to the latest version available
--from=FROM_REPOSITORY from repository context
--to=TO_REPOSITORY     to repository context

Description:
list will iterate over all dockerfiles in all branches in the repository and print out all container
image references and list newer versions if available.

check will do the same, and if there are newer versions available print the out of date container
image references and exit with 1.

bump will update the container images references on the specified branch and commit/push the changes
back to the repository.

move will move the container image reference on the specified branch from one registry to another. The
changes are committed/pushed back to the git repository.
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

	if fromage.List || fromage.Check {
		if err := fromage.ForEachDockerfile(ListAllReferences); err != nil {
			log.Fatal(err)
		}

		if fromage.Check {
			fromage.references = fromage.references.FilterOutOfDate()
		}

		if fromage.OnlyReferences {
			fromage.references.OutputOnlyReferences(fromage.Format, fromage.NoHeader)
		} else {
			fromage.references.Output(fromage.Format, fromage.NoHeader)
		}

		if fromage.Check && len(fromage.references) > 0 {
			os.Exit(1)
		}
	} else if fromage.Bump {
		if err := fromage.ForEachDockerfile(BumpReferences); err != nil {
			log.Fatal(err)
		}
		msg := "container image references bumped"
		if fromage.pin != nil {
			msg = msg + " pinned on " + strings.ToLower(fromage.pin.String()) + " level"
		}
		if err := fromage.CommitAndPush(msg); err != nil {
			log.Fatal(err)
		}
	} else if fromage.Move {
		if fromage.From == "" || fromage.To == "" {
			log.Fatal("both --from and --to are required to move an image reference")
		}
		if strings.ContainsAny(fromage.From, "@:") || strings.ContainsAny(fromage.To, "@:") {
			log.Fatal("the --from and --to image references should not contain an tag or digest")
		}
		if err := fromage.ForEachDockerfile(MoveImageReferences); err != nil {
			log.Fatal(err)
		}
		if err := fromage.CommitAndPush(fmt.Sprintf("moved references from %s to %s", fromage.From, fromage.To)); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatalf("I don't know what to do")
	}
}

func (f *Fromage) CommitAndPush(msg string) error {
	if !f.updated {
		return nil
	}
	log.Printf("INFO: %s", msg)
	if !f.DryRun {
		hash, err := f.workTree.Commit(msg, &git.CommitOptions{
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
		var progress io.Writer = os.Stderr
		if !f.Verbose {
			progress = &bytes.Buffer{}
		}
		log.Printf("INFO: pushing changes to %s", f.Url)

		auth, _, err := GetAuth(f.Url)
		if err != nil {
			return err
		}
		return f.repository.Push(&git.PushOptions{Auth: auth, Progress: progress})
	} else {
		log.Printf("INFO: changes would be pushed to %s", f.Url)
	}
	return nil
}
