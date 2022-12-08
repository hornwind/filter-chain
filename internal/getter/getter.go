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
	localCTX, cancel := context.WithCancel(ctx)
	c.fnCancelRunCTX = cancel
	go c.run(localCTX)
}

func (c *Getter) run(ctx context.Context) {
	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()
	log.Debug("Getter started")
	// fill bd on start
	for _, target := range c.targets {
		// TODO https://stackoverflow.com/questions/62387307/how-to-catch-errors-from-goroutines
		go c.updateCountryData(ctx, target)
	}

	for {
		select {
		case <-ctx.Done():
			log.Debugf("Getter ctx: %v", ctx.Err())
			return
		case <-ticker.C:
			for _, target := range c.targets {
				// TODO https://stackoverflow.com/questions/62387307/how-to-catch-errors-from-goroutines
				go c.updateCountryData(ctx, target)
			}
		}
	}
}

func (c *Getter) updateCountryData(ctx context.Context, countryCode string) error {
	if c.countryMustUpdate(countryCode) {
		log.Debugf("Update data for country %s", countryCode)
		if err := c.getRIPECountryData(ctx, countryCode); err != nil {
			log.Error(err)
			return err
		}
		return nil
	}
	log.Debugf("Country %s no need to update", countryCode)
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
	log.Debugf("Get URL: %s", url)
	// TODO with context https://golang.cafe/blog/golang-context-with-timeout-example.html
	resp, err := http.Get(url)
	if err != nil {
		log.Warn(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get data from RIPE with url %s failed", url)
	}

	var respJson RespJson

	jsonDataFromHttp, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Warn(err)
	}

	err = json.Unmarshal([]byte(jsonDataFromHttp), &respJson)
	if err != nil {
		log.Warn("Getter response unmarshal to struct err: ", err)
	}

	time := time.Now()
	cr := &models.IpsetResources{
		Name:            countryCode,
		UpdateTimestamp: time,
		Asn:             respJson.Data.Resources.Asn,
		Ipv4:            respJson.Data.Resources.Ipv4,
		Ipv6:            respJson.Data.Resources.Ipv6,
	}

	return c.storage.CreateOrUpdate(cr)
}
