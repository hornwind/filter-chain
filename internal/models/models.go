package models

import (
	"time"
)

type CountryResources struct {
	Country         string
	UpdateTimestamp time.Time
	Asn             []string
	Ipv4            []string
	Ipv6            []string
}

type Repository interface {
	CreateOrUpdate(c *CountryResources) error
	GetCountryResources(country string) (*CountryResources, error)
	GetCountryTimestamp(country string) (time.Time, error)
	GetCountryAppliedStatus(country string) (bool, error)
	SetCountryApplied(country string) error
	// TODO
	// Delete(ctx context.Context, c CountryResources) (error)
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
