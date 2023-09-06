package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-github/v47/github"
	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, githubCmd)
}

var githubCmd = &cli.Command{
	Name:            "gh",
	Usage:           "GitHub helpers.",
	SkipFlagParsing: true,
	Subcommands: []*cli.Command{
		githubCoauthoredByCommand,
		githubPurgeArtifactsCommand,
		githubPurgeReleaseCommand,
	},
}

type GitHubUserInfoResponse struct {
	ID      uint64
	Login   string
	Blog    *string
	Name    *string
	Company *string
	Email   *string
}

var githubCoauthoredByCommand = &cli.Command{
	Name:    "co-authored-by",
	Aliases: []string{"cab"},
	Action: func(c *cli.Context) error {
		users := c.Args().Slice()
		for i := 0; i < len(users); i++ {
			resp, err := http.Get("https://api.github.com/users/" + users[i])
			if err != nil {
				panic(err)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				panic(err)
			}
			var u GitHubUserInfoResponse
			err = json.Unmarshal(body, &u)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Co-authored-by: %s <%d+%s@users.noreply.github.com>\n", u.Login, u.ID, u.Login)
		}
		return nil
	},
}

var githubPurgeArtifactsCommand = &cli.Command{
	Name:    "purge-artifacts",
	Aliases: []string{"pa"},
	Usage:   "Purge the artifacts of the user or organization.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "access-token",
			Aliases: []string{"t"},
		},
		&cli.StringFlag{
			Name:    "user",
			Aliases: []string{"u"},
		},
		&cli.StringSliceFlag{
			Name:    "ignore",
			Aliases: []string{"e"},
		},
	},
	Action: func(ctx *cli.Context) error {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: ctx.String("access-token")},
		)
		tc := oauth2.NewClient(ctx.Context, ts)
		client := github.NewClient(tc)
		var opts = &github.RepositoryListOptions{}
		var allRepos []*github.Repository
		for opts != nil {
			repos, resp, err := client.Repositories.List(ctx.Context, "", opts)
			if err != nil {
				return err
			}
			if resp.NextPage > 0 {
				opts.ListOptions.Page = resp.NextPage
			} else {
				opts = nil
			}
			allRepos = append(allRepos, repos...)
		}

		ignores := ctx.StringSlice("ignore")
		var ignoresMap = make(map[string]bool)
		for _, ignore := range ignores {
			ignoresMap[strings.ToLower(ignore)] = true
		}

		for _, repo := range allRepos {
			repoName := fmt.Sprintf("%s/%s", *repo.Owner.Login, *repo.Name)
			if ignoresMap[strings.ToLower(repoName)] {
				continue
			}
			if !strings.EqualFold(*repo.Owner.Login, ctx.String("user")) {
				continue
			}
			log.Printf("Processing %s/%s", *repo.Owner.Login, *repo.Name)
			var opts = &github.ListOptions{}
			var allArtifacts []*github.Artifact
			for opts != nil {
				artifacts, resp, err := client.Actions.ListArtifacts(ctx.Context, *repo.Owner.Login, *repo.Name, opts)
				if err != nil {
					return err
				}
				if resp.NextPage > 0 {
					opts.Page = resp.NextPage
				} else {
					opts = nil
				}
				allArtifacts = append(allArtifacts, artifacts.Artifacts...)
			}
			log.Printf("Deleting %d artifacts", len(allArtifacts))
			for _, artifact := range allArtifacts {
				_, err := client.Actions.DeleteArtifact(ctx.Context, *repo.Owner.Login, *repo.Name, *artifact.ID)
				if err != nil {
					return err
				}
			}
			log.Printf("Done %s/%s", *repo.Owner.Login, *repo.Name)
		}
		return nil
	},
}

var githubPurgeReleaseCommand = &cli.Command{
	Name:    "purge-release",
	Aliases: []string{"pr"},
	Usage:   "Purge the release of the user or organization.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "access-token",
			Aliases: []string{"t"},
		},
		&cli.StringFlag{
			Name:    "user",
			Aliases: []string{"u"},
		},
		&cli.StringSliceFlag{
			Name:    "repository",
			Aliases: []string{"r"},
		},
	},
	Action: func(ctx *cli.Context) error {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: ctx.String("access-token")},
		)
		tc := oauth2.NewClient(ctx.Context, ts)
		client := github.NewClient(tc)
		var opts = &github.RepositoryListOptions{}

		var allRepos []*github.Repository
		for opts != nil {
			repos, resp, err := client.Repositories.List(ctx.Context, "", opts)
			if err != nil {
				return err
			}
			if resp.NextPage > 0 {
				opts.ListOptions.Page = resp.NextPage
			} else {
				opts = nil
			}
			allRepos = append(allRepos, repos...)
		}

		repositories := ctx.StringSlice("repository")
		var repositoriesMap = make(map[string]bool)
		for _, repository := range repositories {
			repositoriesMap[strings.ToLower(repository)] = true
		}

		for _, repo := range allRepos {
			repoName := fmt.Sprintf("%s/%s", *repo.Owner.Login, *repo.Name)
			if !repositoriesMap[strings.ToLower(repoName)] {
				continue
			}
			if !strings.EqualFold(*repo.Owner.Login, ctx.String("user")) {
				continue
			}
			log.Printf("Processing %s/%s", *repo.Owner.Login, *repo.Name)

			var opts = &github.ListOptions{}
			var allReleases []*github.RepositoryRelease
			for opts != nil {
				releases, resp, err := client.Repositories.ListReleases(ctx.Context, *repo.Owner.Login, *repo.Name, opts)
				if err != nil {
					return err
				}
				if resp.NextPage > 0 {
					opts.Page = resp.NextPage
				} else {
					opts = nil
				}
				allReleases = append(allReleases, releases...)
			}
			log.Printf("Deleting %d releases", len(allReleases))
			for _, release := range allReleases {
				_, err := client.Repositories.DeleteRelease(ctx.Context, *repo.Owner.Login, *repo.Name, *release.ID)
				if err != nil {
					return err
				}
			}
			log.Printf("Done %s/%s", *repo.Owner.Login, *repo.Name)
		}
		return nil
	},
}
