package strategies

import (
	"context"
	goexmodel "github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex_backtest/sim"
)

type SampleStragtegy struct {
	api  *sim.ExchangeSim
	pair goexmodel.CurrencyPair
}

func NewSampleStrategy(api *sim.ExchangeSim, pair goexmodel.CurrencyPair) *SampleStragtegy {
	return &SampleStragtegy{api: api, pair: pair}
}
func (s *SampleStragtegy) Main(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, _, err := s.api.GetDepth(s.pair, 1)
			if err != nil {
				return
			}
		}
	}
}
