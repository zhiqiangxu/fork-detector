package main

import (
	"context"
	"flag"
	"os"
	"syscall"

	"github.com/oklog/run"
	"github.com/zhiqiangxu/fork-detector/config"
	"github.com/zhiqiangxu/fork-detector/pkg/detector"
	"github.com/zhiqiangxu/fork-detector/pkg/log"
	"github.com/zhiqiangxu/util/signal"
)

var confFile string

func init() {
	flag.StringVar(&confFile, "conf", "./config.json", "configuration file path")
	flag.Parse()
}

func main() {
	log.InitLog(log.InfoLog, "./Log/", log.Stdout)

	conf, err := config.LoadConfig(confFile)
	if err != nil {
		log.Fatalf("LoadConfig fail:%v", err)
	}

	var detectors []detector.Detector
	for _, chain := range conf.Chains {
		d := detector.New(chain.Type, chain)
		detectors = append(detectors, d)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	signal.SetupHandler(func(sig os.Signal) {
		cancelFunc()
	}, syscall.SIGINT, syscall.SIGTERM)

	var g run.Group
	for i := range detectors {
		d := detectors[i]
		g.Add(func() error {
			return d.Start(ctx)
		}, func(error) {
			d.Stop()
		})
	}

	doneCh := make(chan struct{})
	go func() {
		err := g.Run()
		close(doneCh)
		log.Warnf("run.Group finished:%v", err)
	}()

	<-doneCh
}
