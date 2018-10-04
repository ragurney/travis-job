package travis

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

// Job a Travis action's job configuration
type Job struct {
	branch       string `required:"true"`
	client       *http.Client
	repoOwner    string `required:"true"`
	repoName     string `required:"true"`
	travisToken  string `required:"true"`
	travisTLD    string `required:"true"`
	pollInterval int    `required:"true"`
}

type triggerBuildResponse struct {
	Request struct {
		ID json.Number `json:"id"`
	} `json:"request"`
}

type build struct {
	ID            json.Number `json:"id"`
	PreviousState string      `json:"previous_state"`
	State         string      `json:"state"`
}

type buildStatusResponse struct {
	Builds []build `json:"builds"`
}

var travisSuccessTermSet = map[string]struct{}{
	"passed": {},
}

var travisFailureTermSet = map[string]struct{}{
	"failed":   {},
	"errored":  {},
	"canceled": {},
}

var travisDoneTermSet = map[string]struct{}{
	"passed":   {},
	"failed":   {},
	"errored":  {},
	"canceled": {},
}

// NewJob initializes a Travis action's job
func NewJob(branch string, owner string, repoName string, token string, tld string, pi int) *Job {
	zerolog.TimeFieldFormat = ""

	j := Job{
		client:       &http.Client{Timeout: 5 * time.Second},
		branch:       branch,
		repoOwner:    owner,
		repoName:     repoName,
		travisToken:  token,
		travisTLD:    tld,
		pollInterval: pi,
	}

	return &j
}

func (j *Job) triggerBuild() (requestID string, err error) {
	// TODO: make travis action url configurable, e.g. .org vs .com
	url := fmt.Sprintf("https://api.travis-ci.%s/repo/%s%%2F%s/requests", j.travisTLD, j.repoOwner, j.repoName)
	data := []byte(fmt.Sprintf(`{"request": {"branch": %q}}`, j.branch))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Travis-API-Version", "3")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", j.travisToken))

	resp, err := j.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	res := triggerBuildResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return "", err
	}

	return string(res.Request.ID), nil
}

func (j *Job) getBuildStatus(requestID string) (b build, err error) {
	log.Debug().Msgf("JOB - TRAVIS: Fetching build status for request '%s'", requestID)

	url := fmt.Sprintf(
		"https://api.travis-ci.%s/repo/%s%%2F%s/request/%s",
		j.travisTLD,
		j.repoOwner,
		j.repoName,
		requestID,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return build{}, errors.New("JOB - TRAVIS: Error trying to fetch build status")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Travis-API-Version", "3")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", j.travisToken))

	resp, err := j.client.Do(req) // TODO: check response status
	if err != nil {
		return build{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return build{}, err
	}

	res := buildStatusResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return build{}, err
	}

	if len(res.Builds) > 0 {
		return res.Builds[0], nil // Only expect one build for branch
	}
	return build{}, errors.New("no builds found") // TODO: maybe shouldn't be an error
}

func (j *Job) pollForResult(requestID string) (build, error) {
	c := make(chan build, 1)
	sentBuildID := false

	ticker := time.NewTicker(time.Duration(j.pollInterval) * time.Second)
	go func() {
		for range ticker.C {
			log.Debug().Msg("JOB - TRAVIS: Polling for build result...")
			if b, err := j.getBuildStatus(requestID); err != nil {
				log.Error().Msgf("JOB - TRAVIS: %s", err.Error())
			} else {
				if !sentBuildID {
					log.Debug().Msgf(
						"JOB - TRAVIS: Build started: https://travis-ci.%s/%s/%s/builds/%s",
						j.travisTLD,
						j.repoOwner,
						j.repoName,
						b.ID,
					)
					sentBuildID = true
				}
				if contains(travisDoneTermSet, b.State) {
					c <- b
				}
			}
		}
	}()

	select {
	case b := <-c:
		ticker.Stop()
		return b, nil
	case <-time.After(40 * time.Minute): // TODO: make this configurable.
		ticker.Stop()
		return build{}, errors.New("timed out waiting for build result")
	}
}

func (j *Job) reportSuccess(buildID string) {
	log.Debug().Msgf("JOB - TRAVIS: Reporting success for build '%s'.", buildID)

	// report success
	os.Exit(0)
}

func (j *Job) reportFailure(buildID string) {
	log.Debug().Msgf("JOB - TRAVIS: Reporting failure for build '%s'.", buildID)

	// report failure
	os.Exit(1)
}

func (j *Job) reportStatus(buildID string, status string) {
	if contains(travisSuccessTermSet, status) {
		j.reportSuccess(buildID)
	}
	j.reportFailure(buildID)
}

// Execute starts Travis job. If it is a new job (no continuation token present), it first submits a new
// travis build, then reports the build id to CodePipeline. If it is a continuing job, it polls Travis for
// build progress and reports the result back to CodePipeline once it is complete.
func (j *Job) Execute() {
	var err error

	if requestID, err := j.triggerBuild(); err == nil {
		if b, err := j.pollForResult(requestID); err == nil {
			j.reportStatus(string(b.ID), b.State)
		}
	}
	if err != nil {
		log.Fatal().Msgf("JOB - TRAVIS: %s", err.Error())
	}
}

func contains(set map[string]struct{}, item string) bool {
	_, ok := set[item]
	return ok
}
