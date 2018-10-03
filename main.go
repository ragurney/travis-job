package main

import (
	l "github.com/ragurney/travis-job/internal/lib"
	t "github.com/ragurney/travis-job/pkg/travis"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = ""

	branch := l.Env("BRANCH")
	repoOwner := l.Env("REPO_OWNER")
	repoName := l.Env("REPO_NAME")
	travisToken := l.Env("TRAVIS_TOKEN")
	travisTLD := l.Env("TRAVIS_TLD")

	log.Debug().Msg("Starting Travis job...")
	t.NewJob(branch, repoOwner, repoName, travisToken, travisTLD).Execute()

	// Wait for result from job
	select {}
}
