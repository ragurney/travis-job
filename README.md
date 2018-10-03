# travis-job
A simple tool written in Go to kick off, monitor, and report on a single Travis job.

## Usage
Required Environment Variables:
* `BRANCH`
* `REPO_OWNER`
* `REPO_NAME`
* `TRAVIS_TOKEN`
* `TRAVIS_TLD` (e.g. '.org' or '.com')

Optional Environment Variables:
* `POLL_INTERVAL` (in seconds, default: 30)

`go run main.go`