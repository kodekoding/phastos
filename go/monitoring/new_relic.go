package monitoring

import (
	"log"
	"os"

	"github.com/newrelic/go-agent/v3/newrelic"
)

type (
	NewRelicOpts func(relics *newRelic)
	NewRelics    interface {
	}

	newRelic struct {
		appName    string
		licenseKey string
		app        *newrelic.Application
	}
)

func InitNewRelic(opts ...NewRelicOpts) *newRelic {
	var newRelicPlatform = newRelic{
		appName:    os.Getenv("NEW_RELIC_APP_NAME"),
		licenseKey: os.Getenv("NEW_RELIC_LICENSE_KEY"),
	}

	if newRelicPlatform.appName == "" {
		newRelicPlatform.appName = os.Getenv("APP_NAME")
	}

	for _, opt := range opts {
		opt(&newRelicPlatform)
	}

	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(newRelicPlatform.appName),
		newrelic.ConfigLicense(newRelicPlatform.licenseKey),
		newrelic.ConfigAppLogForwardingEnabled(true),
	)
	newRelicPlatform.app = app
	if err != nil {
		log.Fatalln("Failed to connect new relic: ", err.Error())
		return nil
	}

	return &newRelicPlatform
}

func (n *newRelic) GetApp() *newrelic.Application {
	return n.app
}

func WithAppName(appName string) NewRelicOpts {
	return func(relics *newRelic) {
		relics.appName = appName
	}
}

func WithLicenseKey(licenseKey string) NewRelicOpts {
	return func(relics *newRelic) {
		relics.licenseKey = licenseKey
	}
}
