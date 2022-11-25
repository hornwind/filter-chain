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

type RepositoryInterface interface {
	CreateOrUpdate(c *CountryResources) error
	GetCountryResources(country string) (*CountryResources, error)
	GetCountryTimestamp(country string) (time.Time, error)
	GetCountryAppliedStatus(country string) (bool, error)
	SetCountryApplied(country string) error
	// TODO
	// Delete(ctx context.Context, c CountryResources) (error)
}
