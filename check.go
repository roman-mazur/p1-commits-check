package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func main() {
	var (
		teamSize int
		repoUrl  string
		commit   string

		deadline    time.Time
		deadlineSet bool
	)
	flag.IntVar(&teamSize, "team-size", 3, "Expected number of the committers")
	flag.StringVar(&commit, "commit", "", "The tip commit to use for checking")
	flag.Func("deadline", "Task deadline", func(value string) error {
		deadline = DeadlineTime(value)
		deadlineSet = true
		return nil
	})

	flag.Parse()
	if teamSize <= 0 {
		log.Fatalf("Invalid team size: %d", teamSize)
	}
	repoUrl = flag.Arg(0)
	if len(repoUrl) == 0 {
		repoUrl = "https://github.com/roman-mazur/oak"
	}
	if !deadlineSet {
		deadline = DeadlineTime("2021-10-03")
	}
	if len(commit) == 0 {
		log.Fatal("Commit is not defined")
	}

	dir, err := os.MkdirTemp(os.TempDir(), "commits-check")
	if err != nil {
		log.Fatal("Cannot create the temporary directory")
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("Problems removing temp dir %s: %s", dir, err)
		}
	}()

	fmt.Println(deadline) // TODO: Remove.
	log.Printf("Cloning %s", repoUrl)
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:      repoUrl,
		Progress: io.Discard,
	})
	if err != nil {
		log.Fatal("Cannot clone the repo", err)
	}

	co, err := repo.CommitObject(plumbing.NewHash(commit))
	if err != nil {
		log.Fatalf("Commit %s not found: %s", commit, err)
	}

	points := 0

	authors, sequenceGood, mergeAuthors, hasReverts := Traverse(repo, co, teamSize)
	if len(authors) < teamSize {
		log.Printf("TASK 1: PROBLEM => Bad number of authors: %s", authors)
	} else {
		if len(authors) != teamSize {
			log.Printf("NOTE => Too many authors: %s", authors)
		}
		log.Println("TASK 1: OK")
		points++
	}
	if err := CheckServer(dir); err != nil {
		log.Printf("TASK 2: PROBLEM => Server check failed: %s", err)
	} else {
		log.Println("TASK 2: OK")
		points++
	}
	if !sequenceGood {
		log.Printf("TASK 3: PROBLEM => No sequence of non-merge commits by all team members (non-chronological) was found")
	} else {
		log.Println("TASK 3: OK")
		points++
	}
	if len(mergeAuthors) < teamSize {
		log.Printf("TASK 4: PROBLEM with condition 4 => No sufficient merge authors: %s", mergeAuthors)
	} else {
		if len(mergeAuthors) != teamSize {
			log.Printf("NOTE => Too many merge authors: %s", authors)
		}
		log.Println("TASK 4: OK")
		points++
	}
	if !hasReverts {
		log.Printf("TASK 5: PROBLEM => No correct revert commits")
	} else {
		log.Println("TASK 5: OK")
		points++
	}
	if !CheckFmt(dir) {
		log.Printf("FMT BONUS: PROBLEM")
	} else {
		log.Println("FMT BONUS: OK")
		points++
	}

	log.Println("Total points:", points)
}

func DeadlineTime(str string) time.Time {
	dt, err := time.Parse("2006-01-02", str)
	if err != nil {
		log.Fatalf("Invalid deadline %s: %s", str, err)
	}
	return dt.Add(24 * time.Hour)
}

func CheckServer(dir string) error {
	cmd := exec.Command("go", "run", "server.go")
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		if err := cmd.Process.Kill(); err != nil {
			log.Printf("Problems killing the test server (pid %d): %s", cmd.Process.Pid, err)
		}
	}()

	const retryDelay = 500 * time.Millisecond

	check := func() error {
		log.Println("Trying HTTP GET...")
		ctx, cancel := context.WithTimeout(context.Background(), retryDelay*2)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8795/time", nil)
		if err != nil {
			panic(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("unexpected status code %d", resp.StatusCode)
		}
		defer resp.Body.Close()
		var data struct {
			Time time.Time
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return err
		}
		if data.Time.Before(time.Now().Add(-1*time.Hour)) || data.Time.After(time.Now().Add(1*time.Hour)) {
			return fmt.Errorf("wrong time: %s", data.Time)
		}
		return nil
	}

	if check() == nil {
		return nil
	}

	retryTick := time.NewTicker(retryDelay)
	defer retryTick.Stop()
	rc := 0
	for {
		select {
		case <-retryTick.C:
			if err := check(); err == nil {
				return nil
			} else {
				rc++
				if rc == 2 {
					return err
				}
			}
		}
	}
}

func CheckFmt(dir string) bool {
	cmd := exec.Command("go", "fmt", "server.go")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	res := err == nil && len(out) == 0
	if !res {
		log.Println(string(out))
	}
	return res
}
