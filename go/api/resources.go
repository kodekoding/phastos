package api

import (
	"os"
	"strconv"

	"github.com/kodekoding/phastos/v2/go/cache"
	"github.com/kodekoding/phastos/v2/go/database"
	plog "github.com/kodekoding/phastos/v2/go/log"
)

func (app *App) loadResources() {
	log := plog.Get()
	var err error

	// load DB + transactions
	connString := os.Getenv("DATABASE_CONN_STRING_MASTER")
	if connString != "" {
		app.db, err = database.Connect()
		if err != nil {
			log.Fatal().Msgf("Can't load database: %s", err.Error())
		}

		// get transaction from DB
		app.trx = app.db.GetTransaction()
	}

	redisConnection := os.Getenv("REDIS_CONN_STRING")
	if redisConnection != "" {
		redisTimeout, _ := strconv.Atoi(os.Getenv("REDIS_TIMEOUT"))
		redisMaxActive, _ := strconv.Atoi(os.Getenv("REDIS_MAX_ACTIVE"))
		redisMaxIdle, _ := strconv.Atoi(os.Getenv("REDIS_MAX_IDLE"))
		redisMaxRetry, _ := strconv.Atoi(os.Getenv("REDIS_MAX_RETRY"))

		cacheService := cache.New(
			cache.WithAddress(redisConnection),
			cache.WithTimeout(redisTimeout),
			cache.WithMaxActive(redisMaxActive),
			cache.WithMaxIdle(redisMaxIdle),
			cache.WithMaxRetry(redisMaxRetry),
			cache.WithPassword(os.Getenv("REDIS_PASSWORD")),
			cache.WithUsername(os.Getenv("REDIS_USERNAME")),
		)

		app.WrapToApp(cacheService)
	}
}
