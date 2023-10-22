package cron

import (
	"context"
	"fmt"
	"github.com/kodekoding/phastos/v2/go/env"
	helper2 "github.com/kodekoding/phastos/v2/go/helper"
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

	HandlerFunc func(ctx context.Context) *Response

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

func (eg *Engine) RegisterScheduler(pattern string, handler HandlerFunc) {
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

		respChan := make(chan *Response)
		start := time.Now()
		go func() {
			respChan <- handler(ctx)
		}()

		select {
		case <-ctx.Done():
			_ = helper2.SendSlackNotification(context.Background(),
				helper2.NotifMsgType(helper2.NotifWarnType),
				helper2.NotifTitle("Cron Job Failed (Timeout)"),
				helper2.NotifData(map[string]string{
					"date": start.Format("2006-01-02 15:04:05"),
				}),
			)
		case resp := <-respChan:
			var notifType helper2.SentNotifParamOptions
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
				helper2.NotifTitle(fmt.Sprintf("Cron Job %s", msg)),
				helper2.NotifData(notifData),
				notifType,
			)

			log.Printf("%s Insert Default Schedule in %.2f second(s)", msg, end.Seconds())
		}
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
