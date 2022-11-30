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

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		os.Kill,
		syscall.SIGTERM,
	)
	defer stop()

	log.Debug("Start app")

	config, err := config.LoadConfig("./test-data")
	if err != nil {
		log.Fatal("cannot load config:", err)
	}
	log.Debug(fmt.Sprintf("%v", config))

	storage, err := bolt.NewStorage("./test-data/data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

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

	go cg.Run(ctx)

	applier, err := applier.NewApplier(stop, config, storage)
	if err != nil {
		log.Fatalln(err)
	}
	go applier.Run(ctx)

	<-ctx.Done()
}
