package lib

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
)

// Env looks env value for passed in key, logging and failing if not set
func Env(name string) string {
	zerolog.TimeFieldFormat = ""

	v, ok := os.LookupEnv(name)
	if !ok {
		log.Fatal().Str("Name", name).Msg("Environment variable is not set.")
	}
	return v
}
