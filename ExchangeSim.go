package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/nntaoli-project/goex"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	DataFinishedError        = errors.New("depth data finished")
	InsufficientError        = errors.New("insufficient")
	CancelOrderFinishedError = errors.New("order finished")
	NotFoundOrderError       = errors.New("not found order")
	AssetSnapshotCsvFileName = "%s_asset_snapshot.csv"
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

	sortedCurrencies []goex.Currency
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

	for _, sub := range sim.acc.SubAccounts {
		sim.sortedCurrencies = append(sim.sortedCurrencies, sub.Currency)
	}

	sort.Slice(sim.sortedCurrencies, func(i, j int) bool {
		return strings.Compare(sim.sortedCurrencies[i].Symbol, sim.sortedCurrencies[j].Symbol) > 0
	})

	var header []string
	for _, c := range sim.sortedCurrencies {
		header = append(header, c.Symbol)
	}

	csvFile := fmt.Sprintf(AssetSnapshotCsvFileName, sim.name)
	f, err := os.OpenFile(csvFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0744)
	if err != nil {
		panic(err)
	}
	csvW := csv.NewWriter(f)
	csvW.Write(header)
	csvW.Flush()
	f.Close()

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
	ord.AvgPrice = math.Round(ratio*price+(1-ratio)*ord.AvgPrice*100000000) / 100000000

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
	case goex.BUY:
		tradeFee = dealAmount * fee
	}
	tradeFee = math.Floor(tradeFee*100000000) / 100000000

	ord.Fee += tradeFee

	ex.unFrozenAsset(tradeFee, dealAmount, price, *ord)
}

func (ex *ExchangeSim) matchOrder(ord *goex.Order, isTaker bool) {
	switch ord.Side {
	case goex.SELL:
		for idx := 0; idx < len(ex.currDepth.BidList); idx++ {
			bid := ex.currDepth.BidList[idx]
			if bid.Price >= ord.Price && bid.Amount > 0 {
				ex.fillOrder(isTaker, bid.Amount, bid.Price, ord)
				if ord.Status == goex.ORDER_FINISH {
					delete(ex.pendingOrders, ord.OrderID2)
					ex.finishedOrders[ord.OrderID2] = ord
					break
				}
			} else {
				break
			}
		}
	case goex.BUY:
		idx := len(ex.currDepth.AskList) - 1
		for ; idx >= 0; idx-- {
			ask := ex.currDepth.AskList[idx]
			if ask.Price <= ord.Price && ask.Amount > 0 {
				ex.fillOrder(isTaker, ask.Amount, ask.Price, ord)
				if ord.Status == goex.ORDER_FINISH {
					delete(ex.pendingOrders, ord.OrderID2)
					ex.finishedOrders[ord.OrderID2] = ord
					break
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
	//ord.Cid = ord.OrderID2

	err := ex.frozenAsset(ord)
	if err != nil {
		return nil, err
	}

	ex.pendingOrders[ord.OrderID2] = &ord

	ex.matchOrder(&ord, true)

	if ord.OrderID2 == "binance.com.2020-05-29.471" || ord.OrderID2 == " binance.com.2020-05-29.470" {
		log.Println("match after:===debug===", ord.OrderID2, "==", ex.acc)
	}

	var result goex.Order
	DeepCopyStruct(ord, &result)
	return &result, nil
}

func (ex *ExchangeSim) LimitSell(amount, price string, currency goex.CurrencyPair) (*goex.Order, error) {
	ex.Lock()
	defer ex.Unlock()

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
	//ord.Cid = ord.OrderID2

	err := ex.frozenAsset(ord)
	if err != nil {
		return nil, err
	}

	ex.pendingOrders[ord.OrderID2] = &ord

	ex.matchOrder(&ord, true)

	var result goex.Order
	DeepCopyStruct(ord, &result)

	return &result, nil
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

	ex.unFrozenAsset(0, 0, 0, *ord)

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
func (ex *ExchangeSim) frozenAsset(order goex.Order) error {

	switch order.Side {
	case goex.SELL:
		avaAmount := ex.acc.SubAccounts[order.Currency.CurrencyA].Amount
		if avaAmount < order.Amount {
			return InsufficientError
		}
		ex.acc.SubAccounts[order.Currency.CurrencyA] = goex.SubAccount{
			Currency:     order.Currency.CurrencyA,
			Amount:       avaAmount - order.Amount,
			ForzenAmount: ex.acc.SubAccounts[order.Currency.CurrencyA].ForzenAmount + order.Amount,
			LoanAmount:   0,
		}
	case goex.BUY:
		avaAmount := ex.acc.SubAccounts[order.Currency.CurrencyB].Amount
		need := order.Amount * order.Price
		if avaAmount < need {
			return InsufficientError
		}
		ex.acc.SubAccounts[order.Currency.CurrencyB] = goex.SubAccount{
			Currency:     order.Currency.CurrencyB,
			Amount:       avaAmount - need,
			ForzenAmount: ex.acc.SubAccounts[order.Currency.CurrencyB].ForzenAmount + need,
			LoanAmount:   0,
		}
	}

	ex.assetSnapshot()

	return nil
}

//解冻
func (ex *ExchangeSim) unFrozenAsset(fee, matchAmount, matchPrice float64, order goex.Order) {
	assetA := ex.acc.SubAccounts[order.Currency.CurrencyA]
	assetB := ex.acc.SubAccounts[order.Currency.CurrencyB]

	switch order.Side {
	case goex.SELL:
		if order.Status == goex.ORDER_CANCEL {
			ex.acc.SubAccounts[assetA.Currency] = goex.SubAccount{
				Currency:     assetA.Currency,
				Amount:       assetA.Amount + order.Amount - order.DealAmount,
				ForzenAmount: assetA.ForzenAmount - (order.Amount - order.DealAmount),
				LoanAmount:   0,
			}
		} else {
			ex.acc.SubAccounts[assetA.Currency] = goex.SubAccount{
				Currency:     assetA.Currency,
				Amount:       assetA.Amount,
				ForzenAmount: assetA.ForzenAmount - matchAmount,
				LoanAmount:   0,
			}
			ex.acc.SubAccounts[assetB.Currency] = goex.SubAccount{
				Currency:     assetB.Currency,
				Amount:       assetB.Amount + matchAmount*matchPrice - fee,
				ForzenAmount: assetB.ForzenAmount,
			}
		}

	case goex.BUY:
		if order.Status == goex.ORDER_CANCEL {
			unFrozen := (order.Amount - order.DealAmount) * order.Price
			ex.acc.SubAccounts[assetB.Currency] = goex.SubAccount{
				Currency:     assetB.Currency,
				Amount:       assetB.Amount + unFrozen,
				ForzenAmount: assetB.ForzenAmount - unFrozen,
			}
		} else {
			ex.acc.SubAccounts[assetA.Currency] = goex.SubAccount{
				Currency:     assetA.Currency,
				Amount:       assetA.Amount + matchAmount - fee,
				ForzenAmount: assetA.ForzenAmount,
				LoanAmount:   0,
			}
			ex.acc.SubAccounts[assetB.Currency] = goex.SubAccount{
				Currency:     assetB.Currency,
				Amount:       assetB.Amount + matchAmount*(order.Price-matchPrice),
				ForzenAmount: assetB.ForzenAmount - matchAmount*order.Price,
			}
		}
	}

	ex.assetSnapshot()
}

func (ex *ExchangeSim) assetSnapshot() {
	csvFile := fmt.Sprintf(AssetSnapshotCsvFileName, ex.name)
	f, err := os.OpenFile(csvFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0744)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Println("close file error=", err)
		}
	}()

	csvW := csv.NewWriter(f)

	var (
		data []string
	)

	for _, currency := range ex.sortedCurrencies {
		sub := ex.acc.SubAccounts[currency]
		data = append(data, goex.FloatToString(sub.Amount+sub.ForzenAmount, 10))
	}

	csvW.Write(data)
	csvW.Flush()
}
