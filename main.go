package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bradfitz/slice"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/urfave/cli.v1"
)

type Build struct {
	ID        int64  `json:"id"`
	RepoID    int64  `json:"-"`
	Number    int    `json:"number"`
	Parent    int    `json:"parent"`
	Event     string `json:"event"`
	Status    string `json:"status"`
	Enqueued  int64  `json:"enqueued_at"`
	Created   int64  `json:"created_at"`
	Started   int64  `json:"started_at"`
	Finished  int64  `json:"finished_at"`
	Deploy    string `json:"deploy_to"`
	Commit    string `json:"commit"`
	Branch    string `json:"branch"`
	Ref       string `json:"ref"`
	Refspec   string `json:"refspec"`
	Remote    string `json:"remote"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
	Author    string `json:"author"`
	Avatar    string `json:"author_avatar"`
	Email     string `json:"author_email"`
	Link      string `json:"link_url"`
	Signed    bool   `json:"signed"`
	Verified  bool   `json:"verified"`
}

func main() {
	app := cli.NewApp()

	app.Commands = []cli.Command{
		{
			Name:    "integration-tests",
			Aliases: []string{"int"},
			Usage:   "run integration tests",
			Action:  integrationTests,
			Flags: []cli.Flag{
				cli.IntFlag{
					Name: "commit-wait",
					Value: 20,
					Usage: "time in seconds to wait after commit",
				},
			},
		},
		{
			Name:    "stress-tests",
			Aliases: []string{"stress"},
			Usage:   "run stress tests",
			Action:  stressTests,
			Flags: []cli.Flag{
				cli.IntFlag{
					Name: "commits, c",
					Value: 50,
					Usage: "number of commits to create",
				},
				cli.IntFlag{
					Name: "commit-wait",
					Value: 5,
					Usage: "time in seconds to wait after commit",
				},
				cli.IntFlag{
					Name: "start-build",
					Usage: "build to start from",
				},
				cli.IntFlag{
					Name: "last-build",
					Usage: "build to end at",
				},
			},
		},
	}

	app.Flags = []cli.Flag {
		cli.StringFlag{
			Name: "server, s",
			Usage: "Drone Server to test",
			EnvVar: "DRONE_SERVER",
		},
		cli.StringFlag{
			Name: "token, t",
			Usage: "Drone Access Token to use",
			EnvVar: "DRONE_TOKEN",
		},
		cli.StringFlag{
			Name: "repo, r",
			Usage: "Remote repo to test Drone with",
			Value: "drone-dev-test/junk",
			EnvVar: "REPO",
		},
		cli.StringFlag{
			Name: "github-baseurl",
			Usage: "Base API url for github instance",
			EnvVar: "GITHUB_BASEURL",
		},
		cli.StringFlag{
			Name: "github-token",
			Usage: "Token to auth to github with",
			EnvVar: "GITHUB_TOKEN",
		},
		cli.StringFlag{
			Name: "github-author-name",
			Usage: "Name to associate with commits",
			Value: "John Doe",
			EnvVar: "GITHUB_AUTHOR_NAME",
		},
		cli.StringFlag{
			Name: "github-author-email",
			Usage: "Email to associate with commits",
			Value: "johndoe@gmail.com",
			EnvVar: "GITHUB_AUTHOR_EMAIL",
		},
	}

	app.Run(os.Args)
}

func isBranchExist(c *cli.Context, branch string) (bool, error) {
	org, repo, err := parseRepo(c.GlobalString("repo"))
	if err != nil {
		return false, err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: c.GlobalString("github-token")},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)
	client.BaseURL, _ = url.Parse(c.GlobalString("github-baseurl"))

	_, _, err = client.Repositories.GetBranch(org, repo, branch)

	if err == nil {
		return true, nil
	}

	return false, nil
}

func deleteBranch(c *cli.Context, branch string) error {
	org, repo, err := parseRepo(c.GlobalString("repo"))
	if err != nil {
		return err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: c.GlobalString("github-token")},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)
	client.BaseURL, _ = url.Parse(c.GlobalString("github-baseurl"))

	_, err = client.Git.DeleteRef(org, repo, fmt.Sprintf("refs/heads/%s", branch))

	if err != nil {
		return err
	}

	fmt.Printf("Branch Deleted!\n")

	return nil
}

func createBranch(c *cli.Context, branch string) error {
	org, repo, err := parseRepo(c.GlobalString("repo"))
	if err != nil {
		return err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: c.GlobalString("github-token")},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)
	client.BaseURL, _ = url.Parse(c.GlobalString("github-baseurl"))

	masterBranch, _, err := client.Repositories.GetBranch(org, repo, "master")

	if err != nil {
		return err
	}

	repoCommit := masterBranch.Commit

	lastCommit, _, err := client.Git.GetCommit(org, repo, *repoCommit.SHA)
	if err != nil {
		return err
	}

	lastCommitSHA := lastCommit.SHA

	branchRef := fmt.Sprintf("refs/heads/%s", branch)
	ref := &github.Reference{
		Ref: &branchRef,
		Object: &github.GitObject{
			SHA: lastCommitSHA,
		},
	}
	_, _, err = client.Git.CreateRef(org, repo, ref)

	if err != nil {
		return err
	}

	fmt.Printf("Branch Created!\n")

	return nil
}

func commit(c *cli.Context, branch string) error {
	org, repo, err := parseRepo(c.GlobalString("repo"))
	if err != nil {
		return err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: c.GlobalString("github-token")},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)
	client.BaseURL, _ = url.Parse(c.GlobalString("github-baseurl"))

	gitBranch, _, err := client.Repositories.GetBranch(org, repo, branch)

	if err != nil {
		return err
	}

	repoCommit := gitBranch.Commit

	lastCommit, _, err := client.Git.GetCommit(org, repo, *repoCommit.SHA)
	if err != nil {
		return err
	}

	lastTreeSHA := *lastCommit.Tree.SHA

	path := "hello_world.txt"
	mode := "100644"
	content := fmt.Sprintf("Hello.  It is now %s", time.Now())
	newTreeEntries := []github.TreeEntry{
		github.TreeEntry{
			Path: &path,
			Mode: &mode,
			Content: &content,
		},
	}
	newContentTree, _, err := client.Git.CreateTree(org, repo, lastTreeSHA, newTreeEntries)
	if err != nil {
		return err
	}
	newContentTreeSHA := newContentTree.SHA

	authorName := c.GlobalString("github-author-name")
	authorEmail := c.GlobalString("github-author-email")
	commitDate := time.Now()
	message := "commit via test-drone-deployment"
	createCommit := &github.Commit{
		Author: &github.CommitAuthor{
			Name: &authorName,
			Email: &authorEmail,
			Date: &commitDate,
		},
		Message: &message,
		Parents: []github.Commit{*lastCommit},
		Tree: &github.Tree{
			SHA: newContentTreeSHA,
		},
	}
	newCommit, _, err := client.Git.CreateCommit(org, repo, createCommit)
	if err != nil {
		return err
	}
	newCommitSHA := newCommit.SHA

	branchRef := fmt.Sprintf("refs/heads/%s", branch)
	ref := &github.Reference{
		Ref: &branchRef,
		Object: &github.GitObject{
			SHA: newCommitSHA,
		},
	}
	_, _, err = client.Git.UpdateRef(org, repo, ref, false)

	if err != nil {
		return err
	}

	fmt.Printf("Commited!\n")

	return nil
}

func getLastBuild(buildsJson string) int {
	builds := make([]Build,0)
	json.Unmarshal([]byte(buildsJson), &builds)
	slice.Sort(builds, func(i, j int) bool {
		return builds[i].ID > builds[j].ID
	})

	return builds[0].Number
}

func parseRepo(repo string) (string, string, error) {
	orgRepo := strings.Split(repo, "/")

	if len(orgRepo) != 2 {
		return "", "", fmt.Errorf("Could not properly parse %s", repo)
	}

	return orgRepo[0], orgRepo[1], nil
}

func validate(c *cli.Context) error {
	if c.GlobalString("server") == "" {
		return fmt.Errorf("Please provide a Drone Server")
	}
	if c.GlobalString("token") == "" {
		return fmt.Errorf("Please provide a Drone Token")
	}
	if c.GlobalString("github-baseurl") == "" {
		return fmt.Errorf("Please provide a Github BaseURL")
	}
	if c.GlobalString("github-token") == "" {
		return fmt.Errorf("Please provide a Github Token")
	}

	return nil
}
