package disable

import (
	"errors"
	"fmt"
	"os"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/weaveworks/wksctl/cmd/wksctl/profile/constants"
	"github.com/weaveworks/wksctl/pkg/git"
)

var Cmd = &cobra.Command{
	Use:   "disable",
	Short: "disable profile",
	Args:  profileDisableArgs,
	Run:   profileDisableRun,
}

var profileDisableOptions struct {
	repository string
	noCommit   bool
}

func init() {
	Cmd.Flags().StringVarP(&profileDisableOptions.repository, "repository", "", "", "enable profile from the repository")
	Cmd.Flags().BoolVarP(&profileDisableOptions.noCommit, "no-commit", "", false, "no auto commit and push behaviour")
}

func profileDisableArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("profile disable does not require any argument")
	}
	return nil
}

func profileDisableRun(cmd *cobra.Command, args []string) {
	repoUrl, err := cmd.Flags().GetString("repository")
	if err != nil {
		log.Fatal(err)
	}
	if repoUrl == "" {
		log.Fatal(errors.New("profile repository must be specified"))
	}

	if repoUrl == constants.AppDevAlias {
		repoUrl = constants.AppDevRepoURL
	}

	if git.IsGitURL(repoUrl) == false {
		log.Fatal(errors.New("repository is not a Git URL"))
	}

	hostName, repoName, err := git.HostAndRepoPath(repoUrl)
	if err != nil {
		log.Fatal(err)
	}

	clonePath := path.Join(constants.ProfilesStorePrefix, hostName, repoName)
	// clonePath should exist
	if _, err := os.Stat(clonePath); err != nil {
		log.Fatal(err)
	}

	log.Infof("Deleting profile from path %s ...", clonePath)
	if err := os.RemoveAll(clonePath); err != nil {
		log.Fatal(err)
	}
	log.Infof("Deleted profile from path: %s ...", clonePath)

	noCommit, err := cmd.Flags().GetBool("no-commit")
	if err != nil {
		log.Fatal(err)
	}

	// Similar to enable, the default behaviour is auto-commit and push
	if noCommit == false {
		cli := git.NewGitClient(git.ClientParams{})
		log.Info("Removing profile from the local repository ...")
		if err := cli.RmRecursive(clonePath); err != nil {
			log.Fatal(err)
		}
		log.Info("Removed profile from the local repository ...")

		log.Info("Committing the changes ...")
		if err := cli.Commit(fmt.Sprintf("Disable profile: %s", repoUrl), "", ""); err != nil {
			log.Fatal(err)
		}
		log.Info("Committed the changes ...")

		log.Info("Pushing to the remote ...")
		if err := cli.Push(); err != nil {
			log.Fatal(err)
		}
		log.Info("Pushed to the remote ...")
	}

}
