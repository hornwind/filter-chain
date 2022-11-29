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
	// IpsetResources models.IpsetResources
	storage bolt.DB
}

var _ models.Repository = (*Storage)(nil)

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

func (s *Storage) CreateOrUpdate(c *models.IpsetResources) error {
	err := s.storage.Update(func(tx *bolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists([]byte(c.Name))
		if err != nil {
			return fmt.Errorf("could not create %s bucket: %v", c.Name, err)
		}

		// Apply mark
		if err = root.Put([]byte("applied"), []byte("false")); err != nil {
			return fmt.Errorf("could not put timestamp into %s bucket: %v", c.Name, err)
		}

		// Store timestamp
		timestamp, err := json.Marshal(c.UpdateTimestamp)
		if err != nil {
			return fmt.Errorf("could not marshal timestamp json: %v", err)
		}
		if err = root.Put([]byte("timestamp"), []byte(timestamp)); err != nil {
			return fmt.Errorf("could not put timestamp into %s bucket: %v", c.Name, err)
		}

		// Store asns
		asn, err := json.Marshal(c.Asn)
		if err != nil {
			return fmt.Errorf("could not marshal asn json: %v", err)
		}
		if err = root.Put([]byte("asn"), []byte(asn)); err != nil {
			return fmt.Errorf("could not put asns into %s bucket: %v", c.Name, err)
		}

		// Store ipv4
		ipv4, err := json.Marshal(c.Ipv4)
		if err != nil {
			return fmt.Errorf("could not marshal ipv4 json: %v", err)
		}
		if err = root.Put([]byte("ipv4"), []byte(ipv4)); err != nil {
			return fmt.Errorf("could not put ipv4 addresses into %s bucket: %v", c.Name, err)
		}

		// Store ipv6
		ipv6, err := json.Marshal(c.Ipv6)
		if err != nil {
			return fmt.Errorf("could not marshal ipv6 json: %v", err)
		}
		if err = root.Put([]byte("ipv6"), []byte(ipv6)); err != nil {
			return fmt.Errorf("could not put ipv6 addresses into %s bucket: %v", c.Name, err)
		}

		return nil
	})
	return err
}

func (c *Storage) GetIpsetTimestamp(name string) (time.Time, error) {
	var data []byte
	result := time.Now().AddDate(0, 0, -2)
	err := c.storage.View(func(tx *bolt.Tx) error {
		data = tx.Bucket([]byte(name)).Get([]byte("timestamp"))
		if data == nil {
			return fmt.Errorf("could not fetch timestamp for %s", name)
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	if err = json.Unmarshal(data, &result); err != nil {
		log.Warn(err)
	}
	log.Debug(fmt.Sprintf("%s update time: %v", name, result))
	return result, err
}

func (c *Storage) GetIpsetAppliedStatus(name string) (bool, error) {
	var status bool
	var data []byte
	err := c.storage.View(func(tx *bolt.Tx) error {
		data = tx.Bucket([]byte(name)).Get([]byte("applied"))
		if data == nil {
			return fmt.Errorf("could not fetch status for %s", name)
		}
		return nil
	})
	if err != nil {
		return false, err
	}

	if err = json.Unmarshal(data, &status); err != nil {
		log.Warn(err)
	}
	log.Debug(fmt.Sprintf("%s is applied: %v", name, status))
	return status, nil
}

func (c *Storage) SetIpsetApplied(name string) error {
	err := c.storage.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(name))
		if b != nil {
			if err := b.Put([]byte("applied"), []byte("true")); err != nil {
				return fmt.Errorf("could not update status for %s: %v", name, err)
			} else {
				return fmt.Errorf("%s bucket does not exist", name)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("update operation for %s bucket failed: %v", name, err)
	}
	return nil
}

func (c *Storage) GetIpsetResources(name string) (*models.IpsetResources, error) {
	var (
		t  []byte
		a  []byte
		i4 []byte
		i6 []byte

		timestamp time.Time
		asn       []string
		ipv4      []string
		ipv6      []string
	)

	// Fetch data from DB
	err := c.storage.View(func(tx *bolt.Tx) error {
		t = tx.Bucket([]byte(name)).Get([]byte("timestamp"))
		if t == nil {
			return fmt.Errorf("could not fetch timestamp for %s", name)
		}

		a = tx.Bucket([]byte(name)).Get([]byte("asn"))
		if t == nil {
			return fmt.Errorf("could not fetch asn for %s", name)
		}

		i4 = tx.Bucket([]byte(name)).Get([]byte("ipv4"))
		if t == nil {
			return fmt.Errorf("could not fetch ipv4 for %s", name)
		}

		i6 = tx.Bucket([]byte(name)).Get([]byte("ipv6"))
		if t == nil {
			return fmt.Errorf("could not fetch ipv6 for %s", name)
		}

		return nil
	})
	if err != nil {
		log.Error("Fetch name data from BD error:", err)
		return nil, err
	}

	// Unmarsha data
	if err = json.Unmarshal(t, &timestamp); err != nil {
		log.Warn(err)
		timestamp = time.Now().AddDate(0, 0, -2)
	}
	if err = json.Unmarshal(a, &asn); err != nil {
		log.Error(err)
		return nil, err
	}
	if err = json.Unmarshal(i4, &ipv4); err != nil {
		log.Error(err)
		return nil, err
	}
	if err = json.Unmarshal(i6, &ipv6); err != nil {
		log.Error(err)
		return nil, err
	}

	// Fill result struct
	cr := &models.IpsetResources{
		Name:            name,
		UpdateTimestamp: timestamp,
		Asn:             asn,
		Ipv4:            ipv4,
		Ipv6:            ipv6,
	}

	return cr, nil
}
