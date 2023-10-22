package cron

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/robfig/cron/v3"
)

type (
	Options func(*option)
	option  struct {
		timezone string
	}

	handlerFunc func(ctx context.Context)

	Engine struct {
		engine       *cron.Cron
		handlerTotal int
	}
)

func New(configOptions ...Options) *Engine {
	var options option
	for _, opt := range configOptions {
		opt(&options)
	}

	var cronOpts cron.Option
	if options.timezone != "" {
		location, err := time.LoadLocation(options.timezone)
		if err != nil {
			log.Fatalln("Failed to load location to run scheduler: ", err.Error())
		}

		cronOpts = cron.WithLocation(location)
	}
	scheduler := cron.New(cronOpts)
	return &Engine{engine: scheduler}
}

func WithTimeZone(timeZone string) Options {
	return func(c *option) {
		c.timezone = timeZone
	}
}

func (eg *Engine) RegisterScheduler(pattern string, handler handlerFunc) {
	if eg.engine == nil {
		log.Fatalln("cron engine is nil")
	}
	if _, err := eg.engine.AddFunc(pattern, func() {
		timeoutProcessEnv := os.Getenv("CRON_JOB_TIMEOUT_PROCESS")
		if timeoutProcessEnv == "" {
			timeoutProcessEnv = "5"
		}

		timeoutProcess, _ := strconv.Atoi(timeoutProcessEnv)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutProcess)*time.Minute)
		defer cancel()
		handler(ctx)
	}); err != nil {
		log.Fatalln("Failed to Register Scheduler Handler: ", err.Error())
	}
	eg.handlerTotal++
}

func (eg *Engine) Start() {
	log.Printf("Cron Job / Scheduler is running, handle %d handler(s)", eg.handlerTotal)
	eg.engine.Start()
}

func (eg *Engine) Stop() {
	log.Println("Cron Job / Scheduler stopped")
	eg.engine.Stop()
}
