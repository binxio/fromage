package main

import (
	"fmt"
	sshconfig "github.com/kevinburke/ssh_config"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

func getCredentialHelper(url string) string {
	cmd := exec.Command("git", "config", "--get-urlmatch", "credential.helper", url)
	helper, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(helper))
	}

	if exiterr, ok := err.(*exec.ExitError); ok {
		if exiterr.ExitCode() != 1 {
			log.Fatalf("ERROR: %s returned exitcode %d", cmd.String(), exiterr.ExitCode())
		}
	} else {
		log.Fatalf("ERROR: %s failed %s", cmd.String(), err)
	}
	return ""
}

func getPassword(repositoryUrl string) transport.AuthMethod {

	u, err := url.Parse(repositoryUrl)
	if err != nil {
		log.Fatalf("ERROR: url '%s' could not be parsed, %s", repositoryUrl, err)
	}

	if os.Getenv("GIT_ASKPASS") == "" && getCredentialHelper(repositoryUrl) == "" {
		// No credential helper specified, not passing in credentials
		return nil
	}

	user := u.User.Username()
	password, _ := u.User.Password()

	cmd := exec.Command("git", "credential", "fill")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("ERROR: internal error on getPassword %s", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("ERROR: internal error on getPassword %s", err)
	}

	go func() {
		defer stdin.Close()
		input := fmt.Sprintf("protocol=%s\nhost=%s\nusername=%s\npath=%s\n", u.Scheme, u.Host, user, u.Path)
		io.WriteString(stdin, input)
	}()

	out, err := cmd.Output()
	if err != nil {
		io.Copy(os.Stderr, stderr)
		log.Fatalf("ERROR: git credential fill failed, %s", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		value := strings.SplitN(line, "=", 2)
		if value[0] == "username" {
			user = value[1]
		}
		if value[0] == "password" {
			password = value[1]
		}
	}

	return &githttp.BasicAuth{Username: user, Password: password}
}

func GetAuth(url string) (auth transport.AuthMethod, plainOpen bool, err error) {

	if MatchesScheme(url) {
		if os.Getenv("GIT_ASKPASS") != "" || getCredentialHelper(url) != "" {
			auth = getPassword(url)
		}
	} else if MatchesScpLike(url) {
		user, host, _, _ := FindScpLikeComponents(url)

		if user == "" {
			user = sshconfig.Get(host, "User")
		}
		keyFile := sshconfig.Get(host, "IdentityFile")
		keyFile, _ = homedir.Expand(keyFile)

		sshKey, err := ioutil.ReadFile(keyFile)
		if err != nil {
			return nil, false, fmt.Errorf("ERROR: failed to read key file '%s', %s", keyFile, err)
		}

		signer, err := ssh.ParsePrivateKey(sshKey)
		if err != nil {
			return nil, false, fmt.Errorf("ERROR: failed to read private key from '%s', %s", keyFile, err)
		}

		auth = &gitssh.PublicKeys{User: user, Signer: signer}

	} else {
		auth = nil
		plainOpen = true
	}
	return
}

func Clone(url string, progress io.Writer) (r *git.Repository, err error) {
	var plainOpen bool
	var auth transport.AuthMethod

	if auth, plainOpen, err = GetAuth(url); err != nil {
		return nil, err
	}

	if plainOpen {
		r, err = git.PlainOpenWithOptions(url, &git.PlainOpenOptions{DetectDotGit: true})
		if err != nil {
			return nil, err
		}
	} else {
		r, err = git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
			URL:      url,
			Progress: progress,
			Auth:     auth,
			Depth:    2,
		})

		if err != nil {
			return nil, err
		}
		err = r.Fetch(&git.FetchOptions{
			RefSpecs: []config.RefSpec{"refs/*:refs/*"},
			Depth:    1,
			Auth:     auth,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return nil, fmt.Errorf("ERROR: failed to fetch all branches from %s, %s", url, err)
		}
	}

	return r, nil
}
