package detector

import (
	"context"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zhiqiangxu/fork-detector/config"
	"github.com/zhiqiangxu/fork-detector/pkg/log"
	"github.com/zhiqiangxu/lru"
)

type Eth struct {
	chain   config.Chain
	stopCh  chan struct{}
	clients []*ethclient.Client
	cache   lru.Cache
}

func newEth(chain config.Chain) *Eth {
	return &Eth{stopCh: make(chan struct{}), chain: chain, cache: lru.NewCache(1000, 0, nil)}
}

func (d *Eth) init() (err error) {
	var clients []*ethclient.Client
	for _, node := range d.chain.URLs {
		client, err := ethclient.Dial(node)
		if err != nil {
			log.Fatalf("ethclient.Dial failed:%v", err)
		}

		clients = append(clients, client)
	}
	d.clients = clients
	return
}

func (d *Eth) Start(ctx context.Context) (err error) {

	err = d.init()
	if err != nil {
		return
	}

	ticker := time.NewTicker(time.Second * 2)

	var nextHeight uint64
	for {
		select {
		case <-ticker.C:
			idx := randIdx(len(d.clients))
			client := d.clients[idx]
			height, err := ethGetCurrentHeight(d.chain.URLs[idx])
			if err != nil {
				log.Warnf("ethGetCurrentHeight failed:%v, chain:%s", err, d.chain.Name)
				continue
			}
			if nextHeight == 0 {
				header, err := client.HeaderByNumber(context.Background(), big.NewInt(int64(height)))
				if err != nil {
					log.Warnf("HeaderByNumber failed:%v, chain:%s", err, d.chain.Name)
					continue
				}
				d.addHeader(header)
				nextHeight = height + 1
				continue
			}

			if height < nextHeight {
				continue
			}

			for nextHeight <= height {
				header, err := client.HeaderByNumber(context.Background(), big.NewInt(int64(nextHeight)))
				if err != nil {
					log.Warnf("HeaderByNumber failed:%v, chain:%s", err, d.chain.Name)
					sleep()
					continue
				}
				parent, ok := d.cache.Get(nextHeight - 1)
				if !ok {
					log.Fatalf("parentHeader(%d) not found, chain:%s", nextHeight-1, d.chain.Name)
				}
				parentHeader := parent.(*types.Header)
				if header.ParentHash != parentHeader.Hash() {
					log.Warnf("fork spotted at height %d, previous:%s, now:%s, chain:%s", nextHeight-1, parentHeader.Hash(), header.ParentHash, d.chain.Name)
					d.updateFork(client, header)
				} else {
					d.addHeader(header)
				}
				nextHeight++
			}
		case <-ctx.Done():
			err = ctx.Err()
			log.Info("quiting from signal...")
			return
		}
	}
}

func (d *Eth) addHeader(header *types.Header) {
	d.cache.Add(header.Number.Uint64(), header, 0)
}

func (d *Eth) getHeaderCache(height uint64) *types.Header {
	h, ok := d.cache.Get(height)
	if !ok {
		return nil
	}
	return h.(*types.Header)
}

type forkPair struct {
	old, new *types.Header
}

func (d *Eth) updateFork(client *ethclient.Client, header *types.Header) {
	var path []forkPair
	cursor := header
	for {
		oldHeader := d.getHeaderCache(cursor.Number.Uint64() - 1)
		if oldHeader == nil {
			log.Infof("fork depth over limit, chain:%s", d.chain.Name)
			break
		}
		if oldHeader.Hash() != cursor.ParentHash {
			newHeader, err := client.HeaderByNumber(context.Background(), big.NewInt(int64(cursor.Number.Uint64()-1)))
			if err != nil {
				log.Warnf("updateFork.HeaderByNumber failed:%v, chain:%s", err, d.chain.Name)
				sleep()
				continue
			}

			if newHeader.Hash() != cursor.ParentHash {
				if newHeader.Hash() == oldHeader.Hash() {
					break
				}
			}
			path = append(path, forkPair{old: oldHeader, new: newHeader})
			d.addHeader(newHeader)
			cursor = newHeader
		} else {
			break
		}
	}
	d.addHeader(header)

	if len(path) == 0 {
		return
	}

	startPair := path[len(path)-1]
	endPair := path[0]
	log.Warnf(
		"fork size:%d, start height:%d, start hash:(new:%s, old:%s), end height:%d, end hash:(new:%s, old:%s), chain:%s",
		len(path),
		startPair.new.Number.Uint64(),
		startPair.new.Hash(),
		startPair.old.Hash(),
		endPair.new.Number.Uint64(),
		endPair.new.Hash(),
		endPair.old.Hash(),
		d.chain.Name)

}

func sleep() {
	time.Sleep(time.Second)
}
func randIdx(size int) int {
	return int(rand.Uint32()) % size
}

func (d *Eth) Stop() {
	close(d.stopCh)
}
