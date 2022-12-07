package models

import (
	"time"
)

type IpsetResources struct {
	Name            string    `json:"name"`
	UpdateTimestamp time.Time `json:"timestamp"`
	Asn             []string  `json:"asn"`
	Ipv4            []string  `json:"ipv4"`
	Ipv6            []string  `json:"ipv6"`
}

type Repository interface {
	CreateOrUpdate(c *IpsetResources) error
	GetIpsetResources(name string) (*IpsetResources, error)
	GetIpsetTimestamp(name string) (time.Time, error)
	GetBoolKV(bucket, key string) (bool, error)
	SetBoolKV(bucket, key string, val bool) error
	// TODO
	// Delete(ctx context.Context, c IpsetResources) (error)
}

type Firewall interface {
	// EnsureChain checks if the specified chain exists and, if not, creates it.  If the chain existed, return true.
	EnsureChain(table, chain, policy string) (bool, error)
	// DeleteChain deletes the specified chain.  If the chain did not exist, return error.
	DeleteChain(table, chain string) error
	// EnsureRule checks if the specified rule is present and, if not, creates it.  If the rule existed, return true.
	EnsureRule(pos int, table, chain string, rulespec ...string) (bool, error)
	// DeleteRule checks if the specified rule is present and, if so, deletes it.
	DeleteRule(table, chain string, rulespec ...string) error
}
