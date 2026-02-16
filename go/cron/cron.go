package cron

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/kodekoding/phastos/v2/go/env"
	helper2 "github.com/kodekoding/phastos/v2/go/helper"
	"github.com/rs/zerolog/log"

	"github.com/robfig/cron/v3"
)

type (
	Engines interface {
		RegisterScheduler(pattern string, handler HandlerFunc)
		RemoveScheduler(pattern string)
		Start()
		Stop()
		Wrap(wrapper Wrapper)
	}
	Options func(*option)
	option  struct {
		timezone string
	}

	HandlerFunc func(ctx context.Context) *Response

	Engine struct {
		engine       *cron.Cron
		handlerTotal int
		wrapper      []Wrapper
		ctx          context.Context
		handlerList  map[string]cron.EntryID
	}

	Wrapper interface {
		WrapToContext(ctx context.Context) context.Context
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
			log.Fatal().Err(err).Msg("Failed to load location to run scheduler: ")
		}

		cronOpts = cron.WithLocation(location)
	}
	scheduler := cron.New(cronOpts)
	return &Engine{
		engine: scheduler,
		ctx:    context.Background(),
	}
}

func WithTimeZone(timeZone string) Options {
	return func(c *option) {
		c.timezone = timeZone
	}
}

func (eg *Engine) RegisterScheduler(pattern string, handler HandlerFunc) {
	if eg.engine == nil {
		log.Fatal().Msg("engine is nil")
	}

	entryID, err := eg.engine.AddFunc(pattern, func() {
		eg.wrapperCronHandler(pattern, handler)
	})
	if err != nil {
		log.Fatal().Msgf("Failed to Register Scheduler Handler: %s", err.Error())
	}

	eg.handlerList[pattern] = entryID
	eg.handlerTotal++
}

func (eg *Engine) RemoveScheduler(pattern string) {
	if eg.engine == nil {
		log.Fatal().Msg("cron engine is nil")
	}
	entryID, ok := eg.handlerList[pattern]
	if !ok {
		log.Error().
			Str("process_name", "[REMOVE-SCHEDULER] ").
			Str("pattern", pattern).
			Msg("Pattern isn't registered")
		return
	}

	// removing the scheduler id
	eg.engine.Remove(entryID)
	delete(eg.handlerList, pattern)
	eg.handlerTotal--
	log.Info().
		Str("process_name", "[REMOVE-SCHEDULER]").
		Int("handler_total", eg.handlerTotal).
		Str("pattern", pattern).
		Msg("scheduler successfully removed")

}

func (eg *Engine) wrapperCronHandler(pattern string, handler HandlerFunc) {
	timeoutProcessEnv := os.Getenv("CRON_JOB_TIMEOUT_PROCESS")
	if timeoutProcessEnv == "" {
		timeoutProcessEnv = "1"
	}

	timeoutProcess, _ := strconv.Atoi(timeoutProcessEnv)
	ctx, cancel := context.WithTimeout(eg.ctx, time.Duration(timeoutProcess)*time.Minute)
	defer cancel()

	respChan := make(chan *Response)
	start := time.Now()
	go func() {
		_ = helper2.SendSlackNotification(ctx,
			helper2.NotifMsgType(helper2.NotifInfoType),
			helper2.NotifTitle(fmt.Sprintf("Cron Job %s Started", pattern)),
			helper2.NotifData(map[string]string{
				"date": start.Format("2006-01-02 15:04:05"),
			}),
		)
		log.Info().
			Str("process_name", "[SCHEDULER-RUN]").
			Str("pattern", pattern).Msg("Cron Job Started")

		respChan <- handler(ctx)
	}()

	select {
	case <-ctx.Done():
		end := time.Since(start)
		_ = helper2.SendSlackNotification(ctx,
			helper2.NotifMsgType(helper2.NotifWarnType),
			helper2.NotifTitle(fmt.Sprintf("Cron Job %s Failed (Timeout)", pattern)),
			helper2.NotifData(map[string]string{
				"Processing Time": fmt.Sprintf("%.2f second(s)", end.Seconds()),
			}),
		)
	case resp := <-respChan:
		var notifType helper2.SentNotifParamOptions
		notifType = helper2.NotifMsgType(helper2.NotifInfoType)
		var msg = "Success"
		notifChannel := os.Getenv("NOTIFICATION_SLACK_INFO_WEBHOOK")
		end := time.Since(start)
		notifData := map[string]string{
			"Processing Time": fmt.Sprintf("%.2f second(s)", end.Seconds()),
			"Environment":     env.ServiceEnv(),
			"Process Name":    resp.processName,
		}
		if resp.err != nil {
			notifType = helper2.NotifMsgType(helper2.NotifErrorType)
			msg = "Error"
			notifChannel = os.Getenv("NOTIFICATIONS_SLACK_WEBHOOK_URL")
			notifData["error"] = resp.err.Error()
		}

		_ = helper2.SendSlackNotification(ctx,
			helper2.NotifChannel(notifChannel),
			helper2.NotifTitle(fmt.Sprintf("Cron Job %xs %s", pattern, msg)),
			helper2.NotifData(notifData),
			notifType,
		)

		log.Info().
			Str("process_name", "[SCHEDULER-RUN]").
			Str("pattern", pattern).
			Str("status", msg).
			Float64("execution_time", end.Seconds()).
			Msg("Cron Job Finished")
	}
}

func (eg *Engine) Wrap(wrapper Wrapper) {
	eg.wrapper = append(eg.wrapper, wrapper)
}

func (eg *Engine) Start() {
	for _, wrapper := range eg.wrapper {
		eg.ctx = wrapper.WrapToContext(eg.ctx)
	}
	log.Info().Int("handler", eg.handlerTotal).Msg("Cron Job / Scheduler is running")
	eg.engine.Start()
}

func (eg *Engine) Stop() {
	log.Info().Msg("Cron Job / Scheduler stopped")
	eg.engine.Stop()
}
