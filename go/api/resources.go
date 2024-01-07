package api

import (
	"os"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/kodekoding/phastos/v2/go/cache"
	"github.com/kodekoding/phastos/v2/go/database"
)

func (app *App) loadResources() {
	var err error

	// load DB + transactions
	app.db, err = database.Connect()
	if err != nil {
		log.Fatal().Msgf("Can't load database: %s", err.Error())
	}

	// get transaction from DB
	app.trx = app.db.GetTransaction()

	redisConnection := os.Getenv("REDIS_CONN_STRING")
	if redisConnection != "" {
		redisTimeout, _ := strconv.Atoi(os.Getenv("REDIS_TIMEOUT"))
		redisMaxActive, _ := strconv.Atoi(os.Getenv("REDIS_MAX_ACTIVE"))
		redisMaxIdle, _ := strconv.Atoi(os.Getenv("REDIS_MAX_IDLE"))

		app.cache = cache.New(
			cache.WithAddress(redisConnection),
			cache.WithTimeout(redisTimeout),
			cache.WithMaxActive(redisMaxActive),
			cache.WithMaxIdle(redisMaxIdle),
			cache.WithPassword(os.Getenv("REDIS_PASSWORD")),
			cache.WithUsername(os.Getenv("REDIS_USERNAME")),
		)
	}
}
