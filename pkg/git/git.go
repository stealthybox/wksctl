package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	giturls "github.com/whilp/git-urls"
)

// Client can perform git operations on the given directory
type Client struct {
	envVars []string
	dir     string
}

// ClientParams groups the arguments to provide to create a new Git client.
type ClientParams struct {
	PrivateSSHKeyPath string
}

// Options holds options for cloning a git repository
type Options struct {
	URL    string
	Branch string
	User   string
	Email  string
}

// ValidateURL validates the URL field of this Options object, returning an
// error should the current value not be valid.
func (o Options) ValidateURL() error {
	if o.URL == "" {
		return errors.New("empty Git URL")
	}
	if !IsGitURL(o.URL) {
		return errors.New("invalid Git URL")
	}
	if !o.isSSHURL() {
		return errors.New("got a HTTP(S) Git URL, but eksctl currently only supports SSH Git URLs")
	}
	return nil
}

func (o Options) isSSHURL() bool {
	url, err := giturls.Parse(o.URL)
	return err == nil && (url.Scheme == "git" || url.Scheme == "ssh")
}

// NewGitClient returns a client that can perform git operations
func NewGitClient(params ClientParams) *Client {
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = ""
	}
	return &Client{
		dir:     workingDir,
		envVars: envVars(params),
	}
}

func envVars(params ClientParams) []string {
	envVars := []string{}
	if params.PrivateSSHKeyPath != "" {
		envVars = append(envVars, fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s", params.PrivateSSHKeyPath))
	}
	return envVars
}

// CloneOptions are the options for cloning a Git repository
type CloneOptions struct {
	URL      string
	Revision string
}

func (git Client) exec(command string, dir string, args ...string) error {
	cmd := exec.Command(command, args...)
	if len(git.envVars) > 0 {
		cmd.Env = git.envVars
	}
	// TODO enable when debug only
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	return cmd.Run()
}

func (git Client) runGitCmd(args ...string) error {
	log.Debugf("running git %v in %s", args, git.dir)
	return git.exec("git", git.dir, args...)
}

// Add performs can perform a `git add` operation on the given file paths
func (git Client) Add(files ...string) error {
	args := append([]string{"add", "--"}, files...)
	if err := git.runGitCmd(args...); err != nil {
		return err
	}
	return nil
}

// RmRecursive performs can perform a `git rm` operation on the given file paths
func (git Client) RmRecursive(files ...string) error {
	args := append([]string{"rm", "-r", "--"}, files...)
	if err := git.runGitCmd(args...); err != nil {
		return err
	}
	return nil
}

// Commit makes a commit if there are staged changes
func (git Client) Commit(message, user, email string) error {
	// Note, this used to do runGitCmd(diffCtx, git.dir, "diff", "--cached", "--quiet", "--", fi.opts.gitFluxPath); err == nil {
	if err := git.runGitCmd("diff", "--cached", "--quiet"); err == nil {
		log.Info("Nothing to commit (the repository contained identical files), moving on")
		return nil
	} else if _, ok := err.(*exec.ExitError); !ok {
		return err
	}

	// If the username and email have been provided, configure and use these as
	// otherwise, git will rely on the global configuration, which may lead to
	// confusion at best, as a different username/email will be used, or if
	// missing (e.g.: in CI, in a blank environment), will fail with:
	//   *** Please tell me who you are.
	//   [...]
	//   fatal: unable to auto-detect email address (got '[...]')
	// N.B.: we do it before committing, instead of after cloning, as other
	// operations will not fail because of missing configuration, and as we may
	// commit on a repository we haven't cloned ourselves.
	if email != "" {
		if err := git.runGitCmd("config", "user.email", email); err != nil {
			return err
		}
	}
	if user != "" {
		if err := git.runGitCmd("config", "user.name", user); err != nil {
			return err
		}
	}

	args := []string{"commit", "-m", message}
	if user == "" && email == "" {
		log.Info("Use default user name and password ...")
	} else {
		args = append(args, fmt.Sprintf("--author=%s <%s>", user, email))
	}

	if err := git.runGitCmd(args...); err != nil {
		return err
	}
	return nil
}

// Push pushes the changes to the origin remote
func (git Client) Push() error {
	err := git.runGitCmd("push")
	return err
}

// CloneRepoInPath behaves like CloneRepoInTmpDir but clones the repository in a specific directory
// which creates if needed
func (git *Client) CloneRepoInPath(clonePath string, options CloneOptions) error {
	if err := os.MkdirAll(clonePath, 0700); err != nil {
		return errors.Wrapf(err, "unable to create directory for cloning")
	}
	return git.cloneRepoInPath(clonePath, options)
}

func (git *Client) cloneRepoInPath(clonePath string, options CloneOptions) error {
	// we do shallow clone
	args := []string{"clone", options.URL, clonePath}
	if err := git.runGitCmd(args...); err != nil {
		return err
	}
	// Set the working directory to the cloned directory, but
	// only do it after the clone so that it doesn't create an
	// undesirable nested directory
	git.dir = clonePath

	if options.Revision != "" {
		// Switch to target branch
		args := []string{"checkout", options.Revision}
		if err := git.runGitCmd(args...); err != nil {
			return err
		}
	}
	return nil
}

func (git *Client) isRepoEmpty() (bool, error) {
	// A repository is empty if it doesn't have branches
	files, err := ioutil.ReadDir(filepath.Join(git.dir, ".git", "refs", "heads"))
	if err != nil {
		return false, err
	}
	return len(files) == 0, nil
}

// HostAndRepoPath returns the host name and the name of the repository given its URL
func HostAndRepoPath(repoURL string) (string, string, error) {
	url, err := giturls.Parse(repoURL)
	if err != nil {
		return "", "", errors.Wrapf(err, "unable to parse git URL '%s'", repoURL)
	}

	return url.Hostname(), strings.TrimRight(url.Path, ".git"), nil
}

// IsGitURL returns true if the argument matches the git url format
func IsGitURL(rawURL string) bool {
	parsedURL, err := giturls.Parse(rawURL)
	if err == nil && parsedURL.IsAbs() && parsedURL.Hostname() != "" {
		return true
	}
	return false
}
