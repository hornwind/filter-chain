package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hornwind/filter-chain/internal/applier"
	"github.com/hornwind/filter-chain/internal/getter"
	"github.com/hornwind/filter-chain/internal/models/repository/bolt"
	"github.com/hornwind/filter-chain/pkg/config"
	_ "github.com/hornwind/filter-chain/pkg/log"
	log "github.com/sirupsen/logrus"
)

const (
	dbPath = "/var/lib/filter-chain"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		os.Kill,
		syscall.SIGTERM,
	)
	defer stop()

	log.Debug("Start app")

	if err := os.Mkdir(dbPath, 0700); !os.IsExist(err) {
		log.Errorf("Could not access to db path %s", dbPath)
	}

	storage, err := bolt.NewStorage(fmt.Sprintf("%s/%s", dbPath, "data.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	config, err := config.LoadConfig(dbPath)
	if err != nil {
		log.Fatal("cannot load config:", err)
	}
	log.Debugf("%v", config)

	interval, err := time.ParseDuration(config.RefreshInterval)
	if err != nil {
		log.Warn(err)
		log.Warn("Use 12h interval for update")
		interval = time.Duration(12 * time.Hour)
	}

	countryCodes := config.CountryDenyList
	countryCodes = append(countryCodes, config.CountryAllowList...)
	log.Debug(countryCodes)

	cg, err := getter.NewGetter(stop, countryCodes, interval, storage)
	if err != nil {
		log.Fatalln(err)
	}
	cg.Run(ctx)

	applier, err := applier.NewApplier(stop, config, storage)
	if err != nil {
		log.Fatalln(err)
	}
	applier.Run(ctx)

	<-ctx.Done()
}
