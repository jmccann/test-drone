package main

import (
	"fmt"
	"time"

	"github.com/verdverm/frisby"
	"gopkg.in/urfave/cli.v1"
)

func integrationTests(c *cli.Context) error {
	fmt.Printf("Testing %s with Frisby!\n", c.GlobalString("server"))
	// fmt.Printf("Token: %s\n", c.GlobalString("token"))
	// fmt.Printf("Github Token: %s\n", c.GlobalString("github-token"))

	validate(c)

	frisby.Global.PrintProgressDot = false

	frisby.Create(fmt.Sprintf("Test GET %s/login homepage", c.GlobalString("server"))).
		Get(fmt.Sprintf("%s/login", c.GlobalString("server"))).
		Send().
		ExpectStatus(200)

	frisby.Create(fmt.Sprintf("Test GET homepage (no token)", c.GlobalString("server"))).
		Get(fmt.Sprintf("%s", c.GlobalString("server"))).
		Send().
		ExpectStatus(200).
		ExpectContent("<!DOCTYPE html>")

	frisby.Create(fmt.Sprintf("Test GET homepage (with token)", c.GlobalString("server"))).
		Get(fmt.Sprintf("%s?access_token=%s", c.GlobalString("server"), c.GlobalString("token"))).
		Send().
		ExpectStatus(200).
		ExpectContent(",\"login\":")

	buildsJson, _ := frisby.Create(fmt.Sprintf("GET last build from %s", c.GlobalString("repo"))).
		Get(fmt.Sprintf("%s/api/repos/%s/builds?access_token=%s", c.GlobalString("server"), c.GlobalString("repo"), c.GlobalString("token"))).
		Send().
		ExpectStatus(200).
		ExpectContent("[").
		Resp.Text()

	lastBuild := getLastBuild(buildsJson)

	exist, err := isBranchExist(c, "junk")
	if err != nil {
		return err
	}
	if exist {
		err = deleteBranch(c, "junk")
	}
	if err != nil {
		return err
	}
	err = createBranch(c, "junk")
	if err != nil {
		return err
	}
	err = commit(c, "junk")
	if err != nil {
		return err
	}
	newBuild := lastBuild + 1

	fmt.Printf("Waiting %d seconds for webhook to trigger and build to finish\n", c.Int("commit-wait"))
	time.Sleep(time.Duration(c.Int("commit-wait")) * time.Second)

	frisby.Create(fmt.Sprintf("GET new build from %s", c.GlobalString("repo"))).
		Get(fmt.Sprintf("%s/api/repos/%s/builds?access_token=%s", c.GlobalString("server"), c.GlobalString("repo"), c.GlobalString("token"))).
		Send().
		ExpectStatus(200).
		ExpectContent(fmt.Sprintf("\"number\":%d,", newBuild))

	frisby.Create(fmt.Sprintf("Check secrets are injected and NOT concealed %s", c.GlobalString("repo"))).
		Get(fmt.Sprintf("%s/api/repos/%s/logs/%d/1?access_token=%s", c.GlobalString("server"), c.GlobalString("repo"), newBuild, c.GlobalString("token"))).
		Send().
		ExpectStatus(200).
		ExpectContent("Not Concealed: MYSUPERSECRETsecret")

	frisby.Create(fmt.Sprintf("Check secrets are injected and ARE concealed in %s", c.GlobalString("repo"))).
		Get(fmt.Sprintf("%s/api/repos/%s/logs/%d/1?access_token=%s", c.GlobalString("server"), c.GlobalString("repo"), newBuild, c.GlobalString("token"))).
		Send().
		ExpectStatus(200).
		ExpectContent("Concealed: **")

	// frisby.Create(fmt.Sprintf("Check secrets are interpolated in %s", c.GlobalString("repo"))).
	//   Get(fmt.Sprintf("%s/api/repos/%s/logs/%d/1?access_token=%s", c.GlobalString("server"), c.GlobalString("repo"), newBuild, c.GlobalString("token"))).
	//   Send().
	//   ExpectStatus(200).
	//   ExpectContent("Interpolation of Secret ($ {}): MYSUPERSECRETsecret")

	// frisby.Create(fmt.Sprintf("Check secrets are interpolated in %s", c.GlobalString("repo"))).
	//   Get(fmt.Sprintf("%s/api/repos/%s/logs/%d/1?access_token=%s", c.GlobalString("server"), c.GlobalString("repo"), newBuild, c.GlobalString("token"))).
	//   Send().
	//   ExpectStatus(200).
	//   ExpectContent("Interpolation of Secret ($): MYSUPERSECRETsecret")

	frisby.Create(fmt.Sprintf("Can restart last job in %s", c.GlobalString("repo"))).
		Post(fmt.Sprintf("%s/api/repos/%s/builds/%d?access_token=%s", c.GlobalString("server"), c.GlobalString("repo"), lastBuild, c.GlobalString("token"))).
		Send().
		ExpectStatus(202)

	frisby.Global.PrintReport()

	if len(frisby.Global.Errors()) > 0 {
		return fmt.Errorf("Errors found")
	}

	return nil
}
