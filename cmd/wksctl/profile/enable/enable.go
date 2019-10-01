package enable

import (
	"errors"
	"fmt"
	"os"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/weaveworks/wksctl/pkg/git"
)

var Cmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable profile",
	Args:  profileEnableArgs,
	Run:   profileEnableRun,
}

var profileEnableOptions struct {
	repository string
	revision   string
	noCommit   bool
}

func init() {
	Cmd.Flags().StringVarP(&profileEnableOptions.repository, "repository", "", "", "enable profile from the repository")
	Cmd.Flags().StringVarP(&profileEnableOptions.revision, "revision", "", "master", "use this revision of the profile")
	Cmd.Flags().BoolVarP(&profileEnableOptions.noCommit, "no-commit", "", false, "no auto commit and push behaviour")
}

const ProfilesStorePrefix = "profiles"

func profileEnableArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("profile does not require any argument")
	}
	return nil
}

func profileEnableRun(cmd *cobra.Command, args []string) {
	repoUrl, err := cmd.Flags().GetString("repository")
	if err != nil {
		log.Fatal(err)
	}
	if repoUrl == "" {
		log.Fatal(errors.New("profile repository must be specified"))
	}

	if repoUrl == "app-dev" {
		repoUrl = "git@github.com:weaveworks/eks-quickstart-app-dev"
	}

	revision, err := cmd.Flags().GetString("revision")
	if err != nil {
		log.Fatal(err)
	}

	if git.IsGitURL(repoUrl) == false {
		log.Fatal(errors.New("repository is not a Git URL"))
	}

	profileCli := git.NewGitClient(git.ClientParams{})
	hostName, repoName, err := git.HostAndRepoPath(repoUrl)
	if err != nil {
		log.Fatal(err)
	}

	clonePath := path.Join(ProfilesStorePrefix, hostName, repoName)
	log.Infof("Cloning into %q ...", clonePath)
	err = profileCli.CloneRepoInPath(clonePath, git.CloneOptions{
		URL:      repoUrl,
		Revision: revision,
	})
	if err != nil {
		log.Fatal(err)
	}

	err = os.RemoveAll(path.Join(clonePath, ".git"))
	if err != nil {
		log.Fatal(err)
	}
	log.Info("Removing .git directory ...")

	noCommit, err := cmd.Flags().GetBool("no-commit")
	if err != nil {
		log.Fatal(err)
	}

	// The default behaviour is auto-commit and push
	if noCommit == false {
		// Need the different git client
		localCli := git.NewGitClient(git.ClientParams{})

		log.Infof("Adding profile %s to the local repository ...", repoUrl)
		log.Infof("Adding clonePath %s to the local repository ...", clonePath)
		if err := localCli.Add(clonePath); err != nil {
			log.Fatal(err)
		}
		log.Infof("Added profile from %s ...", repoUrl)

		log.Info("Committing the profile ...")
		if err := localCli.Commit(fmt.Sprintf("Enable profile: %s", repoUrl), "", ""); err != nil {
			log.Fatal(err)
		}
		log.Info("Committed the profile ...")

		log.Info("Pushing to the remote ...")
		if err := localCli.Push(); err != nil {
			log.Fatal(err)
		}
		log.Info("Pushed successfully.")
	}
}
