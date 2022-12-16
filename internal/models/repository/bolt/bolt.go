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
	storage *bolt.DB
}

var _ models.Repository = (*Storage)(nil)

func NewStorage(path string) (*Storage, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	s := &Storage{
		storage: db,
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
		status, err := json.Marshal(false)
		if err != nil {
			log.Warn("Marshal init bool status error: ", err)
		}
		if err = root.Put([]byte("applied"), []byte(status)); err != nil {
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

func (s *Storage) GetIpsetTimestamp(name string) (time.Time, error) {
	var data []byte
	var result time.Time
	faketime := time.Now().AddDate(0, 0, -2)

	log.Debugf("fetch %s from db", name)
	err := s.storage.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(name))
		if b == nil {
			return nil
		}
		data = b.Get([]byte("timestamp"))
		if data == nil {
			return fmt.Errorf("could not fetch timestamp for %s", name)
		}
		return nil
	})
	if err != nil {
		return faketime, err
	}

	if data == nil {
		log.Warn("Timestamp data from db is nil, return faketime")
		return faketime, nil
	}

	if data != nil {
		if err = json.Unmarshal(data, &result); err != nil {
			log.Warn("Unmarshaled time from db: ", result)
			log.Warn("Could not unmarshal timestamp from db: ", err)
			return faketime, err
		}
	}
	log.Debugf("%s update time: %v", name, result)
	return result, err
}

func (s *Storage) GetIpsetResources(name string) (*models.IpsetResources, error) {
	var (
		t  []byte
		a  []byte
		i4 []byte
		i6 []byte

		timestamp time.Time
	)
	asn := make([]string, 0, 1024)
	ipv4 := make([]string, 0, 1024)
	ipv6 := make([]string, 0, 1024)

	// Fetch data from DB
	err := s.storage.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(name))
		if b == nil {
			return fmt.Errorf("bucket %s is not exist", name)
		}
		t = b.Get([]byte("timestamp"))
		if t == nil {
			return fmt.Errorf("could not fetch timestamp for %s", name)
		}

		a = b.Get([]byte("asn"))
		if t == nil {
			return fmt.Errorf("could not fetch asn for %s", name)
		}

		i4 = b.Get([]byte("ipv4"))
		if t == nil {
			return fmt.Errorf("could not fetch ipv4 for %s", name)
		}

		i6 = b.Get([]byte("ipv6"))
		if t == nil {
			return fmt.Errorf("could not fetch ipv6 for %s", name)
		}

		return nil
	})
	if err != nil {
		log.Errorf("Fetch name data from BD error: %v", err)
		return nil, err
	}

	// Unmarsha data
	if err := json.Unmarshal(t, &timestamp); err != nil {
		log.Warn(err)
		timestamp = time.Now().AddDate(0, 0, -2)
	}
	if err := json.Unmarshal(a, &asn); err != nil {
		log.Error(err)
		return nil, err
	}
	if err := json.Unmarshal(i4, &ipv4); err != nil {
		log.Error(err)
		return nil, err
	}
	if err := json.Unmarshal(i6, &ipv6); err != nil {
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

func (s *Storage) GetBoolKV(bucket, key string) (bool, error) {
	var status bool
	var data []byte
	err := s.storage.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		data = b.Get([]byte(key))
		return nil
	})
	if err != nil {
		return false, err
	}

	if data == nil {
		log.Warn("Received data from db is nil, return false")
		return false, nil
	}
	if data != nil {
		if err = json.Unmarshal(data, &status); err != nil {
			log.Warn("Unmarshalled status: ", status)
			log.Warn("Could not unmarshal status from db: ", err)
			return false, err
		}
	}

	log.Debugf("Bool %s is %v", key, status)
	return status, nil
}

func (s *Storage) SetBoolKV(bucket, key string, val bool) error {
	err := s.storage.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b != nil {
			status, err := json.Marshal(val)
			if err != nil {
				return err
			}
			log.Debugf("Change %s in bucket %s to %v", key, bucket, val)
			if err := b.Put([]byte(key), []byte(status)); err != nil {
				return fmt.Errorf("could not update %s: %v", key, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("update operation for %s bucket failed: %v", bucket, err)
	}
	return nil
}

func (s *Storage) DeleteBucket(bucket string) error {
	err := s.storage.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(bucket)); err != nil {
			log.Errorf("bucket %s is not exists or it's not a bucket", bucket)
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("deletion for bucket %s failed: %v", bucket, err)
	}
	return nil
}

func (s *Storage) ListBuckets() (map[string]struct{}, error) {
	buckets := make(map[string]struct{}, 1)
	err := s.storage.View(func(tx *bolt.Tx) error {
		err := tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			buckets[string(name)] = struct{}{}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	log.Debugf("list buckets: %v", buckets)
	return buckets, nil
}

func (s *Storage) ListBucketsForDeletion() (map[string]struct{}, error) {
	buckets := make(map[string]struct{}, 1)
	err := s.storage.View(func(tx *bolt.Tx) error {
		err := tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			data := b.Get([]byte("deletion_mark"))
			if data != nil {
				var mark bool
				if err := json.Unmarshal(data, &mark); err != nil {
					log.Warn("Unmarshalled status: ", mark)
					log.Warn("Could not unmarshal status from db: ", err)
					return err
				}
				if mark {
					buckets[string(name)] = struct{}{}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	log.Debugf("This buckets should be deleted: %s", buckets)
	return buckets, nil
}

func (s *Storage) GetRule(bucket, key string) ([]string, error) {
	var rule []string
	var data []byte
	err := s.storage.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		data = b.Get([]byte(key))
		return nil
	})
	if err != nil {
		return nil, err
	}

	if data == nil {
		log.Warn("Received data from db is nil, return nil")
		return nil, nil
	}
	if data != nil {
		if err = json.Unmarshal(data, &rule); err != nil {
			log.Warn("Unmarshalled rule: ", rule)
			log.Warn("Could not unmarshal status from db: ", err)
			return nil, err
		}
	}

	log.Debugf("%s is %v", key, rule)
	return rule, nil
}

func (s *Storage) StoreRule(bucket, key string, rule []string) error {
	err := s.storage.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b != nil {
			status, err := json.Marshal(rule)
			if err != nil {
				return err
			}
			log.Debugf("Change %s in bucket %s to %v", key, bucket, rule)
			if err := b.Put([]byte(key), []byte(status)); err != nil {
				return fmt.Errorf("could not update %s: %v", key, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("update operation for %s bucket failed: %v", bucket, err)
	}
	return nil
}

func (s *Storage) GetStringKV(bucket, key string) (string, error) {
	var out string
	var data []byte
	err := s.storage.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		data = b.Get([]byte(key))
		return nil
	})
	if err != nil {
		return "", err
	}

	if data == nil {
		log.Warn("Received data from db is nil, return false")
		return "", nil
	}
	if data != nil {
		if err = json.Unmarshal(data, &out); err != nil {
			log.Warnf("GetStringKV unmarshalled: %v", out)
			log.Warnf("Could not unmarshal str from db: %v", err)
			return "", err
		}
	}

	log.Debugf("String KV %s is %v", key, out)
	return out, nil
}

func (s *Storage) SetStringKV(bucket, key, val string) error {
	err := s.storage.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b != nil {
			data, err := json.Marshal(val)
			if err != nil {
				return err
			}
			log.Debugf("Change %s in bucket %s to %v", key, bucket, val)
			if err := b.Put([]byte(key), []byte(data)); err != nil {
				return fmt.Errorf("could not update %s: %v", key, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("update operation for %s bucket failed: %v", bucket, err)
	}
	return nil
}
