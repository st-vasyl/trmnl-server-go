package main

import (
	"os"
	"sync"
	"time"
	"trmnl-server-go/pkg/v1/config"
	"trmnl-server-go/pkg/v1/db"
	"trmnl-server-go/pkg/v1/handler"
	"trmnl-server-go/pkg/v1/worker"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var configFile = "config.yaml"

func init() {
	zerolog.TimeFieldFormat = ""
	zerolog.TimestampFunc = func() time.Time {
		return time.Date(2008, 1, 8, 17, 5, 05, 0, time.UTC)
	}
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
}

func main() {
	c, _ := config.GetConfig(configFile)
	if c.Common.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Info().
		Str("config", configFile).
		Str("DB", c.Common.Dbpath).
		Msg("Services running")

	err := db.InitDB(c.Common.Dbpath)
	if err != nil {
		log.Error().
			Str("dbpath", c.Common.Dbpath).
			Err(err).
			Msg("Failed to init DB")
		os.Exit(1)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		handler.Serve("0.0.1", &c)
	}()
	go func() {
		defer wg.Done()
		worker.UpdateData(&c)
	}()
	wg.Wait()
	log.Info().Msg("Services has shut down")
}
