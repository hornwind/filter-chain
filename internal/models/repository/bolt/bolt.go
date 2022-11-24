package bolt

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hornwind/filter-chain/internal/models"
	_ "github.com/hornwind/filter-chain/pkg/log"
	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

type Storage struct {
	// mu sync.RWMutex
	// countryResources models.CountryResources
	storage bolt.DB
}

var _ models.RepositoryInterface = (*Storage)(nil)

func NewStorage(path string) (*Storage, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	s := &Storage{
		// mu:      sync.RWMutex{},
		storage: *db,
	}

	return s, err
}

func (s *Storage) Close() {
	s.storage.Close()
}

func (s *Storage) CreateOrUpdate(c *models.CountryResources) error {
	err := s.storage.Update(func(tx *bolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists([]byte(c.Country))
		if err != nil {
			return fmt.Errorf("could not create %s bucket: %v", c.Country, err)
		}

		// Store timestamp
		timestamp, err := json.Marshal(c.UpdateTimestamp)
		if err != nil {
			return fmt.Errorf("could not marshal timestamp json: %v", err)
		}
		if err = root.Put([]byte("timestamp"), []byte(timestamp)); err != nil {
			return fmt.Errorf("could not put timestamp into %s bucket: %v", c.Country, err)
		}

		// Store asns
		asn, err := json.Marshal(c.Asn)
		if err != nil {
			return fmt.Errorf("could not marshal asn json: %v", err)
		}
		if err = root.Put([]byte("asn"), []byte(asn)); err != nil {
			return fmt.Errorf("could not put asns into %s bucket: %v", c.Country, err)
		}

		// Store ipv4
		ipv4, err := json.Marshal(c.Ipv4)
		if err != nil {
			return fmt.Errorf("could not marshal ipv4 json: %v", err)
		}
		if err = root.Put([]byte("ipv4"), []byte(ipv4)); err != nil {
			return fmt.Errorf("could not put ipv4 addresses into %s bucket: %v", c.Country, err)
		}

		// Store ipv6
		ipv6, err := json.Marshal(c.Ipv6)
		if err != nil {
			return fmt.Errorf("could not marshal ipv6 json: %v", err)
		}
		if err = root.Put([]byte("ipv6"), []byte(ipv6)); err != nil {
			return fmt.Errorf("could not put ipv6 addresses into %s bucket: %v", c.Country, err)
		}

		return nil
	})
	return err
}

func (c *Storage) GetCountryTimestamp(country string) (time.Time, error) {
	var data []byte
	result := time.Now().AddDate(0, 0, -2)
	err := c.storage.View(func(tx *bolt.Tx) error {
		data = tx.Bucket([]byte(country)).Get([]byte("timestamp"))
		if data == nil {
			return fmt.Errorf("could not fetch timestamp for country %s", country)
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	if err = json.Unmarshal(data, &result); err != nil {
		log.Warn(err)
	}
	log.Debug(fmt.Sprintf("Country %s update time: %v", country, result))
	return result, err
}

func (c *Storage) GetCountryResources(country string) (*models.CountryResources, error) {
	//pass
	return nil, nil
}
