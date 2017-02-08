package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
	"gopkg.in/urfave/cli.v1"
)

func stressTests(c *cli.Context) error {
	validate(c)

	fmt.Printf("Stress Testing %s\n", c.GlobalString("server"))
	fmt.Printf("Using github API url %s\n", c.GlobalString("github-baseurl"))

	var lastBuild int
	var numberOfBuilds int

	if c.Int("start-build") != 0 && c.Int("last-build") != 0 {
		lastBuild = c.Int("start-build")
		numberOfBuilds = c.Int("last-build") - c.Int("start-build")
	} else {
		numberOfBuilds = c.Int("commits")

		// Create Branch
		fmt.Printf("Resetting branch\n")
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

		// Commits
		resp, err := http.Get(fmt.Sprintf("%s/api/repos/%s/builds?access_token=%s", c.GlobalString("server"), c.GlobalString("repo"), c.GlobalString("token")))
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		lastBuild = getLastBuild(string(body[:]))

		fmt.Printf("Creating %d commits\n", c.Int("commits"))
		for i := 1; i <= c.Int("commits"); i++ {
			fmt.Printf("[%d/%d] Creating Commit\n", i, c.Int("commits"))
			err = commit(c, "junk")
			if err != nil {
				return err
			}
			fmt.Printf("Waiting %d seconds for webhook to trigger and build to finish\n", c.Int("commit-wait"))
			time.Sleep(time.Duration(c.Int("commit-wait")) * time.Second)
		}
	}

	// Repeat rebuilds
	err := loopRebuilds(c, lastBuild, numberOfBuilds)
	if err != nil {
		return err
	}

	return nil
}

func loopRebuilds(c *cli.Context, startBuild, numberOfBuilds int) error {
	for {
		// Fork builds (not working ... only restarting for now)
		values := url.Values{
			"fork": {"true"},
		}
		for i := 1; i <= numberOfBuilds; i++ {
			build := startBuild + i
			fmt.Printf("Restarting build %d\n", build)
			_, err := http.PostForm(fmt.Sprintf("%s/api/repos/%s/builds/%d?access_token=%s", c.GlobalString("server"), c.GlobalString("repo"), build, c.GlobalString("token")), values)
			if err != nil {
				return err
			}
		}

		err := checkBuilds(c)
		if err != nil {
			return err
		}
	}
}

func checkBuilds(c *cli.Context) error {
	var builds []Build

	builds, err := getBuilds(c.GlobalString("server"), c.GlobalString("token"))
	if err != nil {
		return err
	}

	// Let them finish
	for len(isRunning(builds)) > 0 {
		fmt.Printf("Waiting for builds to finish running: %d running, %d pending\n", len(isRunning(builds)), len(isPending(builds)))

		// Read logs from some running builds
		readLogs(c.GlobalString("server"), c.GlobalString("repo"), c.GlobalString("token"), isRunning(builds))

		// Wait a couple seconds before checking builds and reading logs again
		time.Sleep(2)

		builds, err = getBuilds(c.GlobalString("server"), c.GlobalString("token"))
		if err != nil {
			return err
		}

		fmt.Printf("Waiting for builds to finish running (2): %d running, %d pending\n", len(isRunning(builds)), len(isPending(builds)))
	}

	// Watch for pending with no running
	fmt.Printf("Checking for pending jobs\n")
	if len(isPending(builds)) > 0 {
		fmt.Printf("builds: %v\n", builds)
		return fmt.Errorf("There are still jobs in the queue!")
	}

	return nil
}

func getBuilds(server, token string) ([]Build, error) {
	var builds []Build

	resp, err := http.Get(fmt.Sprintf("%s/api/builds?access_token=%s", server, token))
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return builds, err
	}

	err = json.Unmarshal(body, &builds)
	if err != nil {
		return builds, err
	}

	return builds, nil
}

func readLogs(server, repo, token string, builds []Build) {
	var wg sync.WaitGroup
	wg.Add(len(builds))

	for _, build := range builds {
		rawurl := fmt.Sprintf("%s/ws/logs/%s/%d/1?access_token=%s", server, repo, build.Number, token)
		fmt.Printf("Reading logs from: %s\n", rawurl)

		go func() {
			defer wg.Done()
			_ = getWithWebsocket(rawurl)
		}()

		// Wait .25 second before starting next lod read
		time.Sleep(time.Duration(250000000) * time.Nanosecond)
	}

	wg.Wait()
}

func getWithWebsocket(rawurl string) error {
	ws, err := websocket.Dial(strings.Replace(rawurl, "http", "ws", 1), "", "http://localhost/")
	if err != nil {
		return fmt.Errorf("Failed to connect: %s\n", err)
	}

	var msg = make([]byte, 512)
	_, err = ws.Read(msg)
	if err != nil {
		return fmt.Errorf("Failed to read: %s\n", err)
	}
	fmt.Printf("Receive: %s\n", msg)

	return nil
}

func isPending(builds []Build) []Build {
	var pending []Build
	for _,build := range builds {
		if build.Status == "pending" {
			pending = append(pending, build)
		}
	}

	return pending
}

func isRunning(builds []Build) []Build {
	var running []Build
	for _,build := range builds {
		if build.Status == "running" {
			running = append(running, build)
		}
	}

	return running
}
