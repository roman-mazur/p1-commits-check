package main

import (
	"log"
	"regexp"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func Traverse(repo *git.Repository, commit *object.Commit, teamSize int) (authors []string, sequenceGood bool, mergeAuthors []string, hasReverts bool) {
	var (
		am = make(authorsSet, 4)
		ma = make(authorsSet, 4)

		cs     commitsSequence
		revRef string
	)
	cs.reset(commit)

	start := time.Now()
	traverse(commit, teamSize, am, &cs, ma, &revRef)
	log.Printf("Traversal completed in %s", time.Since(start))

	authors = am.Slice()
	sequenceGood = cs.finished
	mergeAuthors = ma.Slice()
	if revRef != "" {
		co, err := repo.CommitObject(plumbing.NewHash(revRef))
		if err == nil && co != nil {
			hasReverts = true
		}
	}
	return
}

type authorsSet map[string]struct{}

func (as authorsSet) Slice() []string {
	authors := make([]string, len(as))
	i := 0
	for a := range as {
		authors[i] = a
		i++
	}
	return authors
}

type commitsSequence struct {
	start, end *object.Commit
	authors    authorsSet

	lastTs           time.Time
	nonChronological bool

	finished bool
}

func (cs *commitsSequence) reset(co *object.Commit) {
	if cs.finished {
		return
	}
	cs.start = co
	cs.end = nil
	cs.authors = make(map[string]struct{})
	cs.authors[co.Author.Email] = struct{}{}
	cs.lastTs = co.Author.When
}

func (cs *commitsSequence) handle(co *object.Commit, teamSize int) bool {
	if cs.finished {
		return false
	}
	if co.NumParents() > 1 {
		// Merge commit.
		return true
	}
	cs.end = co
	cs.authors[co.Author.Email] = struct{}{}
	if cs.lastTs.Before(co.Author.When) {
		cs.nonChronological = true
	} else {
		cs.lastTs = co.Author.When
	}
	cs.finished = len(cs.authors) == teamSize && cs.nonChronological
	return false
}

var revertPtrn = regexp.MustCompile("[Rr]evert.*\\s+([a-f0-9]{40})")

func ParsecRevertRef(msg string) string {
	if res := revertPtrn.FindAllStringSubmatch(msg, 2); len(res) > 0 {
		return res[0][1]
	}
	return ""
}

func traverse(co *object.Commit, teamSize int, am authorsSet, cs *commitsSequence, ma authorsSet, revertRef *string) {
	if co == nil {
		return
	}
	am[co.Author.Email] = struct{}{}
	merge := cs.handle(co, teamSize)

	if merge {
		ma[co.Author.Email] = struct{}{}
	}

	if *revertRef == "" {
		*revertRef = ParsecRevertRef(co.Message)
	}

	for i := 0; i < co.NumParents(); i++ {
		p, err := co.Parent(i)
		if err != nil {
			panic(err)
		}
		if merge {
			cs.reset(p)
		}
		traverse(p, teamSize, am, cs, ma, revertRef)
	}
}
