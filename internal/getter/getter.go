package getter

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/hornwind/filter-chain/internal/models"
	_ "github.com/hornwind/filter-chain/pkg/log"
	log "github.com/sirupsen/logrus"
)

type Getter struct {
	targets        []string
	fnCancelRunCTX context.CancelFunc
	checkInterval  time.Duration
	storage        models.Repository
	// countryData models.IpsetResources
}

type RespJson struct {
	Data struct {
		Resources struct {
			Asn  []string `json:"asn"`
			Ipv4 []string `json:"ipv4"`
			Ipv6 []string `json:"ipv6"`
		} `json:"resources"`
	} `json:"data"`
}

func NewGetter(localCTX context.CancelFunc, targets []string, checkInterval time.Duration, storage models.Repository) (*Getter, error) {
	getter := &Getter{
		fnCancelRunCTX: localCTX,
		checkInterval:  checkInterval,
		targets:        targets,
		storage:        storage,
	}

	return getter, nil
}

func (c *Getter) Run(ctx context.Context) {
	// TODO add ticker
	localCTX, cancel := context.WithCancel(ctx)
	c.fnCancelRunCTX = cancel
	log.Debug("Getter started")
	_ = localCTX
	for _, target := range c.targets {
		go c.updateCountryData(localCTX, target)
	}
}

func (c *Getter) updateCountryData(ctx context.Context, countryCode string) error {
	if c.countryMustUpdate(countryCode) {
		log.Debug(fmt.Sprintf("Update data for country %s", countryCode))
		c.getRIPECountryData(ctx, countryCode)
		return nil
	}
	log.Debug(fmt.Sprintf("Country %s no need to update", countryCode))
	return nil
}

func (c *Getter) countryMustUpdate(countryCode string) bool {
	lastUpdateTime, err := c.storage.GetIpsetTimestamp(countryCode)
	if err != nil {
		log.Warn(fmt.Sprintf("Something went wrong while getting country last update time: %v", err))
		return true
	}
	return lastUpdateTime.Before(time.Now().AddDate(0, 0, -1))
}

func (c *Getter) getRIPECountryData(ctx context.Context, countryCode string) error {
	url := fmt.Sprintf("https://stat.ripe.net/data/country-resource-list/data.json?resource=%s&v4_format=prefix", countryCode)
	log.Debug("Get URL:", url)
	// TODO with context https://golang.cafe/blog/golang-context-with-timeout-example.html
	resp, err := http.Get(url)
	if err != nil {
		log.Warn(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get data from RIPE with url %s failed", url)
	}
	log.Debug(resp.Body)

	var respJson RespJson

	jsonDataFromHttp, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Warn(err)
	}

	err = json.Unmarshal([]byte(jsonDataFromHttp), &respJson)
	if err != nil {
		log.Warn(err)
	}

	time := time.Now()
	cr := &models.IpsetResources{
		Name:            countryCode,
		UpdateTimestamp: time,
		Asn:             respJson.Data.Resources.Asn,
		Ipv4:            respJson.Data.Resources.Ipv4,
		Ipv6:            respJson.Data.Resources.Ipv6,
	}

	// log.Debug(cr)

	return c.storage.CreateOrUpdate(cr)
}
