package detector

import (
	"context"
	"fmt"

	"github.com/zhiqiangxu/fork-detector/config"
)

const (
	EthTy int = 0
)

type Detector interface {
	Start(ctx context.Context) error
	Stop()
}

func New(ty int, chain config.Chain) Detector {
	switch ty {
	case EthTy:
		return newEth(chain)
	default:
		panic(fmt.Sprintf("unknown type:%d", ty))
	}
}
