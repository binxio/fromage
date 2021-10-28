package main

import (
	"bytes"
	"fmt"
	sshconfig "github.com/kevinburke/ssh_config"
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

func identityFileAuthentication(user string, host string) (auth transport.AuthMethod, err error) {

	var keyFile = sshconfig.Get(host, "IdentityFile")

	if user == "" {
		user = sshconfig.Get(host, "User")
	}

	if _, err = os.Stat(keyFile); os.IsNotExist(err) {
		return nil, nil
	}

	key, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s, %s", keyFile, err)
	}

	signer, parseError := ssh.ParsePrivateKey(key)
	if parseError == nil {
		return &gitssh.PublicKeys{User: user, Signer: signer}, nil
	}

	if missingErr, ok := parseError.(*ssh.PassphraseMissingError); ok {
		return sshAgentAuthentication(user, host, keyFile, missingErr.PublicKey)
	}

	if publicKey, _, _, _, publicKeyParseError := ssh.ParseAuthorizedKey(key); publicKeyParseError == nil {
		return sshAgentAuthentication(user, host, keyFile, publicKey)
	}

	return nil, fmt.Errorf("ERROR: failed to read private key from '%s', %s", keyFile, parseError)
}

func sshAgentAuthentication(user, host, keyFile string, key ssh.PublicKey) (auth transport.AuthMethod, err error) {
	if user == "" {
		user = sshconfig.Get(host, "User")
	}

	sshAuthSocket := os.Getenv("SSH_AUTH_SOCK")
	if sshAuthSocket == "" {
		return nil, nil
	}

	publicKeys, err := gitssh.NewSSHAgentAuth(user)
	if err != nil {
		return nil, fmt.Errorf("ERROR: failed to connect ssh agent, %s", err)
	}

	signers, err := publicKeys.Callback()
	if err != nil {
		return nil, fmt.Errorf("ERROR: failed to obtain keys from ssh agent, %s", err)
	}
	for _, signer := range signers {
		if bytes.Compare(signer.PublicKey().Marshal(), key.Marshal()) == 0 {
			return &gitssh.PublicKeys{User: user, Signer: signer}, nil
		}
	}
	if auth == nil && err == nil {
		log.Printf("WARNING: key for identity file %s not available in ssh agent.", keyFile)
	}

	return nil, nil
}

func GetAuth(url string) (auth transport.AuthMethod, plainOpen bool, err error) {

	if MatchesScheme(url) {
		if os.Getenv("GIT_ASKPASS") != "" || getCredentialHelper(url) != "" {
			auth = getPassword(url)
		}
		return auth, false, nil
	}

	if MatchesScpLike(url) {
		user, host, _, _ := FindScpLikeComponents(url)

		if auth, err = identityFileAuthentication(user, host); err != nil {
			return
		}

	} else {
		auth = nil
		plainOpen = true
	}
	return
}

func Clone(url string, progress io.Writer, readOnly bool) (r *git.Repository, err error) {
	var plainOpen bool
	var auth transport.AuthMethod

	if auth, plainOpen, err = GetAuth(url); err != nil {
		return nil, err
	}

	if plainOpen {
		if !readOnly {
			r, err = git.PlainOpenWithOptions(url, &git.PlainOpenOptions{DetectDotGit: true})
			if err != nil {
				return nil, err
			}
		} else {
			r, err = git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
				URL:      url,
				Progress: progress,
				Depth:    2,
			})
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
