package main

import (
	"errors"
	"github.com/nntaoli-project/goex"
	"sync"
	"time"
)

var (
	DataFinishedError        = errors.New("depth data finished")
	InsufficientError        = errors.New("insufficient")
	CancelOrderFinishedError = errors.New("order finished")
	NotFoundOrderError       = errors.New("not found order")
)

type ExchangeSim struct {
	*sync.RWMutex
	acc                  *goex.Account
	name                 string
	makerFee             float64
	takerFee             float64
	supportCurrencyPairs []goex.CurrencyPair
	pendingOrders        map[string]*goex.Order
	finishedOrders       map[string]*goex.Order
	depthLoader          map[goex.CurrencyPair]*DepthDataLoader
	currDepth            goex.Depth
	idGen                *IdGen
}

func NewExchangeSim(config ExchangeSimConfig) *ExchangeSim {
	sim := &ExchangeSim{
		RWMutex:              new(sync.RWMutex),
		idGen:                NewIdGen(config.ExName),
		name:                 config.ExName,
		makerFee:             config.MakerFee,
		takerFee:             config.TakerFee,
		acc:                  &config.Account,
		supportCurrencyPairs: config.SupportCurrencyPairs,
		pendingOrders:        make(map[string]*goex.Order, 100),
		finishedOrders:       make(map[string]*goex.Order, 100),
		depthLoader:          make(map[goex.CurrencyPair]*DepthDataLoader, 1),
	}

	for _, pair := range config.SupportCurrencyPairs {
		sim.depthLoader[pair] = NewDepthDataLoader(DataConfig{
			Ex:       sim.name,
			Pair:     pair,
			StarTime: config.BackTestStartTime,
			EndTime:  config.BackTestEndTime,
			UnGzip:   config.UnGzip,
			Size:     20,
		})
	}

	return sim
}

func (ex *ExchangeSim) fillOrder(isTaker bool, amount, price float64, ord *goex.Order) {
	dealAmount := 0.0
	remain := ord.Amount - ord.DealAmount
	if remain > amount {
		dealAmount = amount
	} else {
		dealAmount = remain
	}

	ratio := dealAmount / (ord.DealAmount + dealAmount)
	ord.AvgPrice = ratio*price + (1-ratio)*ord.AvgPrice

	ord.DealAmount += dealAmount
	if ord.Amount == ord.DealAmount {
		ord.Status = goex.ORDER_FINISH
	} else {
		if ord.DealAmount > 0 {
			ord.Status = goex.ORDER_PART_FINISH
		}
	}

	fee := ex.makerFee
	if isTaker {
		fee = ex.takerFee
	}

	tradeFee := 0.0
	switch ord.Side {
	case goex.SELL:
		tradeFee = dealAmount * price * fee
		ex.unFrozenAsset(tradeFee, dealAmount, price, ord)
	case goex.BUY:
		tradeFee = dealAmount * fee
		ex.unFrozenAsset(tradeFee, dealAmount, price, ord)
	}

	ord.Fee += tradeFee
}

func (ex *ExchangeSim) matchOrder(ord *goex.Order, isTaker bool) {
	switch ord.Side {
	case goex.SELL:
		for idx := 0; idx < len(ex.currDepth.BidList); idx++ {
			bid := ex.currDepth.BidList[idx]
			if bid.Price >= ord.Price {
				ex.fillOrder(isTaker, bid.Amount, bid.Price, ord)
				if ord.Status == goex.ORDER_FINISH {
					delete(ex.pendingOrders, ord.OrderID2)
					ex.finishedOrders[ord.OrderID2] = ord
				}
			} else {
				break
			}
		}
	case goex.BUY:
		idx := len(ex.currDepth.AskList) - 1
		for ; idx >= 0; idx-- {
			ask := ex.currDepth.AskList[idx]
			if ask.Price <= ord.Price {
				ex.fillOrder(isTaker, ask.Amount, ask.Price, ord)
				if ord.Status == goex.ORDER_FINISH {
					delete(ex.pendingOrders, ord.OrderID2)
					ex.finishedOrders[ord.OrderID2] = ord
				}
			} else {
				break
			}
		}
	}
}

func (ex *ExchangeSim) match() {
	ex.Lock()
	defer ex.Unlock()
	for id, _ := range ex.pendingOrders {
		ex.matchOrder(ex.pendingOrders[id], false)
	}
}

func (ex *ExchangeSim) LimitBuy(amount, price string, currency goex.CurrencyPair) (*goex.Order, error) {
	ex.Lock()
	defer ex.Unlock()

	err := ex.frozenAsset(goex.ToFloat64(amount)*goex.ToFloat64(price), currency.CurrencyB)
	if err != nil {
		return nil, err
	}

	ord := goex.Order{
		Price:     goex.ToFloat64(price),
		Amount:    goex.ToFloat64(amount),
		OrderID2:  ex.idGen.Get(),
		OrderTime: int(time.Now().UnixNano() / int64(time.Millisecond)),
		Status:    goex.ORDER_UNFINISH,
		Currency:  currency,
		Side:      goex.BUY,
		Type:      "limit",
	}
	ord.Cid = ord.OrderID2

	ex.pendingOrders[ord.OrderID2] = &ord

	ex.matchOrder(&ord, true)

	var result goex.Order
	DeepCopyStruct(ord , &result)
	return &result, nil
}

func (ex *ExchangeSim) LimitSell(amount, price string, currency goex.CurrencyPair) (*goex.Order, error) {
	ex.Lock()
	defer ex.Unlock()

	err := ex.frozenAsset(goex.ToFloat64(amount), currency.CurrencyA)
	if err != nil {
		return nil, err
	}

	ord := goex.Order{
		Price:     goex.ToFloat64(price),
		Amount:    goex.ToFloat64(amount),
		OrderID2:  ex.idGen.Get(),
		OrderTime: int(time.Now().UnixNano() / int64(time.Millisecond)),
		Status:    goex.ORDER_UNFINISH,
		Currency:  currency,
		Side:      goex.SELL,
		Type:      "limit",
	}
	ord.Cid = ord.OrderID2
	ex.pendingOrders[ord.OrderID2] = &ord

	ex.matchOrder(&ord, true)

	var result goex.Order
	DeepCopyStruct(ord, &result)

	return &ord, nil
}

func (ex *ExchangeSim) MarketBuy(amount, price string, currency goex.CurrencyPair) (*goex.Order, error) {
	panic("not support")
}

func (ex *ExchangeSim) MarketSell(amount, price string, currency goex.CurrencyPair) (*goex.Order, error) {
	panic("not support")
}

func (ex *ExchangeSim) CancelOrder(orderId string, currency goex.CurrencyPair) (bool, error) {
	ex.Lock()
	defer ex.Unlock()

	ord := ex.finishedOrders[orderId]
	if ord != nil {
		return false, CancelOrderFinishedError
	}

	ord = ex.pendingOrders[orderId]
	if ord == nil {
		return false, NotFoundOrderError
	}

	delete(ex.pendingOrders, ord.OrderID2)

	ord.Status = goex.ORDER_CANCEL
	ex.finishedOrders[ord.OrderID2] = ord

	ex.unFrozenAsset(0, ord.Amount-ord.DealAmount, 0, ord)

	return true, nil
}

func (ex *ExchangeSim) GetOneOrder(orderId string, currency goex.CurrencyPair) (*goex.Order, error) {
	ex.RLock()
	defer ex.RUnlock()

	ord := ex.finishedOrders[orderId]
	if ord == nil {
		ord = ex.pendingOrders[orderId]
	}

	if ord != nil {
		// deep copy
		var result goex.Order
		DeepCopyStruct(ord, &result)

		return &result, nil
	}

	return nil, NotFoundOrderError
}

func (ex *ExchangeSim) GetUnfinishOrders(currency goex.CurrencyPair) ([]goex.Order, error) {
	ex.RLock()
	defer ex.RUnlock()

	var unfinishedOrders []goex.Order
	for _, ord := range ex.pendingOrders {
		unfinishedOrders = append(unfinishedOrders, *ord)
	}

	return unfinishedOrders, nil
}

func (ex *ExchangeSim) GetOrderHistorys(currency goex.CurrencyPair, currentPage, pageSize int) ([]goex.Order, error) {
	panic("not support")
}

func (ex *ExchangeSim) GetAccount() (*goex.Account, error) {
	ex.RLock()
	defer ex.RUnlock()

	var account goex.Account
	account.SubAccounts = make(map[goex.Currency]goex.SubAccount, 2)
	for _, sub := range ex.acc.SubAccounts {
		account.SubAccounts[sub.Currency] = goex.SubAccount{
			Currency:     sub.Currency,
			Amount:       sub.Amount,
			ForzenAmount: sub.ForzenAmount,
		}
	}

	return &account, nil
}

func (ex *ExchangeSim) GetTicker(currency goex.CurrencyPair) (*goex.Ticker, error) {
	ask := ex.currDepth.AskList[len(ex.currDepth.AskList)-1].Price
	bid := ex.currDepth.BidList[0].Price
	return &goex.Ticker{
		Pair: currency,
		Last: (ask + bid) / 2,
		Sell: ask,
		Buy:  bid,
	}, nil
}

func (ex *ExchangeSim) GetDepth(size int, currency goex.CurrencyPair) (*goex.Depth, error) {
	depth := ex.depthLoader[currency].Next()
	if depth == nil {
		return nil, DataFinishedError
	}
	ex.currDepth = *depth
	ex.match()
	return depth, nil
}

func (ex *ExchangeSim) GetKlineRecords(currency goex.CurrencyPair, period, size, since int) ([]goex.Kline, error) {
	panic("not support")
}

func (ex *ExchangeSim) GetTrades(currencyPair goex.CurrencyPair, since int64) ([]goex.Trade, error) {
	panic("not support")
}

func (ex *ExchangeSim) GetExchangeName() string {
	return ex.name
}

//冻结
func (ex *ExchangeSim) frozenAsset(amount float64, currency goex.Currency) error {
	avaAmount := ex.acc.SubAccounts[currency].Amount
	if avaAmount < amount {
		return InsufficientError
	}

	ex.acc.SubAccounts[currency] = goex.SubAccount{
		Currency:     currency,
		Amount:       avaAmount - amount,
		ForzenAmount: ex.acc.SubAccounts[currency].ForzenAmount + amount,
		LoanAmount:   0,
	}

	return nil
}

func (ex *ExchangeSim) unFrozenAsset(fee, filledAmount, price float64, ord *goex.Order) {
	assetA := ex.acc.SubAccounts[ord.Currency.CurrencyA]
	assetB := ex.acc.SubAccounts[ord.Currency.CurrencyB]

	switch ord.Side {
	case goex.SELL:
		ex.acc.SubAccounts[ord.Currency.CurrencyA] = goex.SubAccount{
			Currency:     ord.Currency.CurrencyA,
			Amount:       assetA.Amount,
			ForzenAmount: assetA.ForzenAmount - filledAmount,
			LoanAmount:   0,
		}
		ex.acc.SubAccounts[ord.Currency.CurrencyB] = goex.SubAccount{
			Currency:     ord.Currency.CurrencyB,
			Amount:       assetB.Amount + filledAmount*price - fee,
			ForzenAmount: assetB.ForzenAmount,
		}
	case goex.BUY:
		unFrozen := filledAmount * ord.Price
		ex.acc.SubAccounts[ord.Currency.CurrencyA] = goex.SubAccount{
			Currency:     ord.Currency.CurrencyA,
			Amount:       assetA.Amount + filledAmount - fee,
			ForzenAmount: assetA.ForzenAmount,
			LoanAmount:   0,
		}
		ex.acc.SubAccounts[ord.Currency.CurrencyB] = goex.SubAccount{
			Currency:     ord.Currency.CurrencyB,
			Amount:       assetB.Amount + (unFrozen - filledAmount*price),
			ForzenAmount: assetB.ForzenAmount - unFrozen,
		}
	}
}
