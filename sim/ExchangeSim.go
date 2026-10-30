package sim

import (
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	goex "github.com/nntaoli-project/goex/v2"
	goexmodel "github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex_backtest/loader"
	backtestmodel "github.com/nntaoli-project/goex_backtest/model"
	"github.com/nntaoli-project/goex_backtest/util"
)

var (
	DataFinishedError        = errors.New("depth data finished")
	InsufficientError        = errors.New("insufficient")
	CancelOrderFinishedError = errors.New("order finished")
	NotFoundOrderError       = errors.New("not found order")
)

const AssetSnapshotCsvFileName = "%s_asset_snapshot.csv"

type ExchangeSim struct {
	*sync.RWMutex
	accounts                      map[string]goexmodel.Account
	name                          string
	makerFee, takerFee            float64
	supportCurrencyPairs          []goexmodel.CurrencyPair
	quoteCurrency                 string
	pendingOrders, finishedOrders map[string]*goexmodel.Order
	depthLoader                   map[string]*loader.DepthDataLoader
	klineLoader                   *loader.KLineDataLoader
	currKline                     goexmodel.Kline
	currDepth                     goexmodel.Depth
	idGen                         *util.IdGen
	sortedCurrencies              []string
	backTestDataType              backtestmodel.BackTestDataType
}

var _ goex.IPubRest = (*ExchangeSim)(nil)
var _ goex.IPrvRest = (*ExchangeSim)(nil)

func NewExchangeSim(config backtestmodel.ExchangeSimConfig) *ExchangeSim {
	ex := &ExchangeSim{RWMutex: new(sync.RWMutex), idGen: util.NewIdGen(config.ExName), name: config.ExName, makerFee: config.MakerFee, takerFee: config.TakerFee, accounts: config.Accounts, supportCurrencyPairs: config.SupportCurrencyPairs, quoteCurrency: strings.ToUpper(config.QuoteCurrency), pendingOrders: make(map[string]*goexmodel.Order), finishedOrders: make(map[string]*goexmodel.Order), depthLoader: make(map[string]*loader.DepthDataLoader), backTestDataType: config.BackTestData, klineLoader: loader.NewKLineDataLoader(backtestmodel.DataConfig{Ex: config.ExName, StarTime: config.BackTestStartTime, EndTime: config.BackTestEndTime, UnGzip: config.UnGzip})}
	for coin := range ex.accounts {
		ex.sortedCurrencies = append(ex.sortedCurrencies, coin)
	}
	sort.Strings(ex.sortedCurrencies)
	for _, pair := range config.SupportCurrencyPairs {
		if pair.QuoteSymbol != ex.quoteCurrency {
			panic("the CurrencyPair only one quote currency per backtest")
		}
		ex.depthLoader[pair.Symbol] = loader.NewDepthDataLoader(backtestmodel.DataConfig{Ex: ex.name, Pair: pair, StarTime: config.BackTestStartTime, EndTime: config.BackTestEndTime, UnGzip: config.UnGzip, Size: config.DepthSize})
	}
	ex.writeSnapshotHeader()
	return ex
}

func NewExchangeSimWithTomlConfig(ex string) *ExchangeSim {
	c, err := util.LoadTomlConfig(fmt.Sprintf("%s_sim.toml", ex))
	if err != nil {
		panic("not found toml config")
	}
	return NewExchangeSim(c)
}
func (ex *ExchangeSim) GetName() string         { return ex.name }
func (ex *ExchangeSim) GetExchangeName() string { return ex.name }
func (ex *ExchangeSim) GetExchangeInfo() (map[string]goexmodel.CurrencyPair, []byte, error) {
	result := make(map[string]goexmodel.CurrencyPair, len(ex.supportCurrencyPairs))
	for _, pair := range ex.supportCurrencyPairs {
		result[pair.Symbol] = pair
	}
	return result, nil, nil
}
func (ex *ExchangeSim) NewCurrencyPair(base, quote string, _ ...goexmodel.OptionParameter) (goexmodel.CurrencyPair, error) {
	return goexmodel.CurrencyPair{Symbol: strings.ToUpper(base + quote), BaseSymbol: strings.ToUpper(base), QuoteSymbol: strings.ToUpper(quote)}, nil
}

func (ex *ExchangeSim) GetDepth(pair goexmodel.CurrencyPair, _ int, _ ...goexmodel.OptionParameter) (*goexmodel.Depth, []byte, error) {
	source := ex.depthLoader[pair.Symbol]
	if source == nil {
		return nil, nil, fmt.Errorf("unsupported pair %s", pair.Symbol)
	}
	depth := source.Next()
	if depth == nil {
		return nil, nil, DataFinishedError
	}
	ex.currDepth = *depth
	ex.match()
	return depth, nil, nil
}
func (ex *ExchangeSim) GetTicker(pair goexmodel.CurrencyPair, _ ...goexmodel.OptionParameter) (*goexmodel.Ticker, []byte, error) {
	if len(ex.currDepth.Asks) == 0 || len(ex.currDepth.Bids) == 0 {
		return nil, nil, errors.New("no depth data")
	}
	ask, bid := ex.currDepth.Asks[len(ex.currDepth.Asks)-1].Price, ex.currDepth.Bids[0].Price
	return &goexmodel.Ticker{Pair: pair, Last: (ask + bid) / 2, Sell: ask, Buy: bid}, nil, nil
}
func (ex *ExchangeSim) GetKline(pair goexmodel.CurrencyPair, period goexmodel.KlinePeriod, _ ...goexmodel.OptionParameter) ([]goexmodel.Kline, []byte, error) {
	data, err := ex.klineLoader.Next(pair, period, 1)
	if err != nil {
		return nil, nil, err
	}
	ex.currKline = data[0]
	ex.match()
	return data, nil, nil
}
func (ex *ExchangeSim) GetAccount(coin string) (map[string]goexmodel.Account, []byte, error) {
	ex.RLock()
	defer ex.RUnlock()
	result := make(map[string]goexmodel.Account)
	if coin == "" {
		for k, v := range ex.accounts {
			result[k] = v
		}
	} else if acc, ok := ex.accounts[strings.ToUpper(coin)]; ok {
		result[acc.Coin] = acc
	}
	return result, nil, nil
}

func (ex *ExchangeSim) CreateOrder(pair goexmodel.CurrencyPair, qty, price float64, side goexmodel.OrderSide, orderTy goexmodel.OrderType, _ ...goexmodel.OptionParameter) (*goexmodel.Order, []byte, error) {
	if orderTy != goexmodel.OrderType_Limit {
		return nil, nil, errors.New("only limit orders are supported")
	}
	if qty <= 0 || price <= 0 {
		return nil, nil, errors.New("quantity and price must be positive")
	}
	ex.Lock()
	defer ex.Unlock()
	order := &goexmodel.Order{Pair: pair, Id: ex.idGen.Get(), Side: side, OrderTy: orderTy, Status: goexmodel.OrderStatus_Pending, Price: price, Qty: qty, CreatedAt: ex.timestamp()}
	if err := ex.freeze(order); err != nil {
		return nil, nil, err
	}
	ex.pendingOrders[order.Id] = order
	ex.matchOrder(order, true)
	return copyOrder(order), nil, nil
}
func (ex *ExchangeSim) GetOrderInfo(pair goexmodel.CurrencyPair, id string, _ ...goexmodel.OptionParameter) (*goexmodel.Order, []byte, error) {
	ex.RLock()
	defer ex.RUnlock()
	order := ex.finishedOrders[id]
	if order == nil {
		order = ex.pendingOrders[id]
	}
	if order == nil || order.Pair.Symbol != pair.Symbol {
		return nil, nil, NotFoundOrderError
	}
	return copyOrder(order), nil, nil
}
func (ex *ExchangeSim) GetPendingOrders(pair goexmodel.CurrencyPair, _ ...goexmodel.OptionParameter) ([]goexmodel.Order, []byte, error) {
	ex.RLock()
	defer ex.RUnlock()
	var result []goexmodel.Order
	for _, order := range ex.pendingOrders {
		if order.Pair.Symbol == pair.Symbol {
			result = append(result, *copyOrder(order))
		}
	}
	return result, nil, nil
}
func (ex *ExchangeSim) GetHistoryOrders(pair goexmodel.CurrencyPair, _ ...goexmodel.OptionParameter) ([]goexmodel.Order, []byte, error) {
	ex.RLock()
	defer ex.RUnlock()
	var result []goexmodel.Order
	for _, order := range ex.finishedOrders {
		if order.Pair.Symbol == pair.Symbol {
			result = append(result, *copyOrder(order))
		}
	}
	return result, nil, nil
}
func (ex *ExchangeSim) CancelOrder(pair goexmodel.CurrencyPair, id string, _ ...goexmodel.OptionParameter) ([]byte, error) {
	ex.Lock()
	defer ex.Unlock()
	if ex.finishedOrders[id] != nil {
		return nil, CancelOrderFinishedError
	}
	order := ex.pendingOrders[id]
	if order == nil || order.Pair.Symbol != pair.Symbol {
		return nil, NotFoundOrderError
	}
	delete(ex.pendingOrders, id)
	order.Status = goexmodel.OrderStatus_Canceled
	order.CanceledAt = ex.timestamp()
	ex.finishedOrders[id] = order
	ex.release(order, 0, 0, 0)
	return nil, nil
}

func (ex *ExchangeSim) timestamp() int64 {
	if ex.backTestDataType == backtestmodel.BackTestDataType_KLine {
		return ex.currKline.Timestamp
	}
	return ex.currDepth.UTime.UnixNano() / int64(1e6)
}
func (ex *ExchangeSim) match() {
	ex.Lock()
	defer ex.Unlock()
	for _, order := range ex.pendingOrders {
		ex.matchOrder(order, false)
	}
}
func (ex *ExchangeSim) matchOrder(order *goexmodel.Order, taker bool) {
	if ex.backTestDataType == backtestmodel.BackTestDataType_KLine {
		ex.fillOrder(taker, order.Qty, order.Price, order)
		return
	}
	if order.Side == goexmodel.Spot_Sell {
		for _, bid := range ex.currDepth.Bids {
			if bid.Price < order.Price || bid.Amount <= 0 {
				break
			}
			ex.fillOrder(taker, bid.Amount, bid.Price, order)
			if order.Status == goexmodel.OrderStatus_Finished {
				return
			}
		}
	} else {
		for i := len(ex.currDepth.Asks) - 1; i >= 0; i-- {
			ask := ex.currDepth.Asks[i]
			if ask.Price > order.Price || ask.Amount <= 0 {
				break
			}
			ex.fillOrder(taker, ask.Amount, ask.Price, order)
			if order.Status == goexmodel.OrderStatus_Finished {
				return
			}
		}
	}
}
func (ex *ExchangeSim) fillOrder(taker bool, amount, price float64, order *goexmodel.Order) {
	deal := math.Min(amount, order.Qty-order.ExecutedQty)
	if deal <= 0 {
		return
	}
	total := order.ExecutedQty + deal
	order.PriceAvg = (order.PriceAvg*order.ExecutedQty + price*deal) / total
	order.ExecutedQty = total
	if total == order.Qty {
		order.Status = goexmodel.OrderStatus_Finished
		order.FinishedAt = ex.timestamp()
		delete(ex.pendingOrders, order.Id)
		ex.finishedOrders[order.Id] = order
	} else {
		order.Status = goexmodel.OrderStatus_PartFinished
	}
	rate := ex.makerFee
	if taker {
		rate = ex.takerFee
	}
	fee := deal * rate
	if order.Side == goexmodel.Spot_Sell {
		fee *= price
	}
	order.Fee += fee
	ex.release(order, fee, deal, price)
}
func (ex *ExchangeSim) freeze(order *goexmodel.Order) error {
	coin, required := order.Pair.QuoteSymbol, order.Qty*order.Price
	if order.Side == goexmodel.Spot_Sell {
		coin, required = order.Pair.BaseSymbol, order.Qty
	}
	acc, ok := ex.accounts[coin]
	if !ok || acc.AvailableBalance < required {
		return InsufficientError
	}
	acc.AvailableBalance -= required
	acc.FrozenBalance += required
	ex.accounts[coin] = acc
	return nil
}
func (ex *ExchangeSim) release(order *goexmodel.Order, fee, qty, price float64) {
	base, quote := ex.accounts[order.Pair.BaseSymbol], ex.accounts[order.Pair.QuoteSymbol]
	if order.Status == goexmodel.OrderStatus_Canceled {
		if order.Side == goexmodel.Spot_Sell {
			rest := order.Qty - order.ExecutedQty
			base.AvailableBalance += rest
			base.FrozenBalance -= rest
			ex.accounts[base.Coin] = base
		} else {
			rest := (order.Qty - order.ExecutedQty) * order.Price
			quote.AvailableBalance += rest
			quote.FrozenBalance -= rest
			ex.accounts[quote.Coin] = quote
		}
		return
	}
	if order.Side == goexmodel.Spot_Sell {
		base.FrozenBalance -= qty
		base.Balance -= qty
		quote.AvailableBalance += qty*price - fee
		quote.Balance += qty*price - fee
		ex.accounts[base.Coin] = base
		ex.accounts[quote.Coin] = quote
	} else {
		base.AvailableBalance += qty - fee
		base.Balance += qty - fee
		quote.FrozenBalance -= qty * order.Price
		quote.AvailableBalance += qty * (order.Price - price)
		quote.Balance -= qty * price
		ex.accounts[base.Coin] = base
		ex.accounts[quote.Coin] = quote
	}
}
func copyOrder(order *goexmodel.Order) *goexmodel.Order { result := *order; return &result }

func (ex *ExchangeSim) writeSnapshotHeader() {
	f, err := os.OpenFile(fmt.Sprintf(AssetSnapshotCsvFileName, ex.name), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0744)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	row := make([]string, 0, len(ex.sortedCurrencies)*2+1)
	for _, coin := range ex.sortedCurrencies {
		row = append(row, coin+"_available", coin+"_frozen")
	}
	row = append(row, "NetAsset")
	w := csv.NewWriter(f)
	_ = w.Write(row)
	w.Flush()
}
func (ex *ExchangeSim) AssetSnapshot() {
	f, err := os.OpenFile(fmt.Sprintf(AssetSnapshotCsvFileName, ex.name), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0744)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	row := make([]string, 0, len(ex.sortedCurrencies)*2+1)
	net := 0.0
	for _, coin := range ex.sortedCurrencies {
		acc := ex.accounts[coin]
		row = append(row, strconv.FormatFloat(acc.AvailableBalance, 'f', 10, 64), strconv.FormatFloat(acc.FrozenBalance, 'f', 10, 64))
		if coin == ex.quoteCurrency {
			net += acc.Balance
		} else if ex.backTestDataType == backtestmodel.BackTestDataType_KLine {
			net += acc.Balance * ex.currKline.Close
		} else {
			ticker, _, err := ex.GetTicker(goexmodel.CurrencyPair{BaseSymbol: coin, QuoteSymbol: ex.quoteCurrency})
			if err == nil {
				net += acc.Balance * ticker.Buy
			}
		}
	}
	row = append(row, strconv.FormatFloat(net, 'f', 10, 64))
	w := csv.NewWriter(f)
	_ = w.Write(row)
	w.Flush()
}
