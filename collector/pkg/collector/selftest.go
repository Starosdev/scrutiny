package collector

import (
	"net/url"

	"github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/sirupsen/logrus"
)

type SelfTestCollector struct {
	BaseCollector

	apiEndpoint *url.URL
	logger      *logrus.Entry
}

// CreateSelfTestCollector creates a new SelfTestCollector with auth support.
func CreateSelfTestCollector(appConfig config.Interface, logger *logrus.Entry, apiEndpoint string) (SelfTestCollector, error) {
	apiEndpointUrl, err := url.Parse(apiEndpoint)
	if err != nil {
		return SelfTestCollector{}, err
	}

	timeout := 60
	if appConfig != nil && appConfig.IsSet("api.timeout") {
		timeout = appConfig.GetAPITimeout()
	}

	apiToken := ""
	if appConfig != nil {
		apiToken = appConfig.GetAPIToken()
	}

	stc := SelfTestCollector{
		BaseCollector: BaseCollector{
			logger:     logger,
			httpClient: NewAuthHTTPClient(timeout, apiToken),
		},
		apiEndpoint: apiEndpointUrl,
		logger:      logger,
	}

	return stc, nil
}

func (sc *SelfTestCollector) Run() error {
	return nil
}
