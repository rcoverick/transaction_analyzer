package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"time"
)

// config holds configurable values
// that can be set to define how the program runs,
// what stats are computed and how they're aggregated,
// what file(s) to read transactions from, etc..
type config struct {
	TransactionsFile string `json:"transactionsFile"`
}

// transaction represents a transaction from a TD Ameritrade
// account transaction log.
type transaction struct {
	Date        time.Time
	Description string
	Quantity    *big.Float
	Symbol      string
	Price       *big.Float
	Commission  *big.Float
	Amount      *big.Float
}

// newTransactionTDA constructs a new transaction struct
// from a csv row in a transaction log downloaded from
// TD Ameritrade.
func newTransactionTDA(r []string) (*transaction, error) {

	quantity, _, err := big.ParseFloat(r[3], 10, 53, big.ToNearestEven)
	if err != nil {
		quantity = big.NewFloat(0) // sane default
	}

	price, _, err := big.ParseFloat(r[5], 10, 53, big.ToNearestEven)
	if err != nil {
		price = big.NewFloat(0)
	}

	commission, _, err := big.ParseFloat(r[6], 10, 53, big.ToNearestEven)
	if err != nil {
		commission = big.NewFloat(0)
	}

	amount, _, err := big.ParseFloat(r[7], 10, 53, big.ToNearestEven)
	if err != nil {
		amount = big.NewFloat(0)
	}

	dtFormat := "01/02/2006"
	transactionDt, err := time.Parse(dtFormat, r[0])
	if err != nil {
		return nil, err
	}

	t := transaction{
		Date:        transactionDt,
		Description: r[2],
		Symbol:      r[4],
		Quantity:    quantity,
		Price:       price,
		Commission:  commission,
		Amount:      amount}
	// make quantity negative if not a 'buy' transaction
	if !strings.HasPrefix(t.Description, "Bought") {
		t.Quantity.Neg(quantity)
	}

	return &t, nil

}

// newConfig returns a new instance of a
// config struct with default values populated
func newConfig() *config {
	c := config{}
	c.TransactionsFile = "transactions.csv"
	return &c
}

// getConfigs loads the configurations from the file named
// "config.json" in the same directory as the executable.
func getConfigs() (*config, error) {
	configFile, err := os.Open("config.json")
	config := newConfig()
	if err != nil {
		// use sane default configurations
		return config, err
	} else {
		defer configFile.Close()
	}
	bytes, err := ioutil.ReadAll(configFile)

	if err != nil {
		return config, err
	}

	json.Unmarshal(bytes, &config)

	return config, err
}

// loadTransactions loads the csv transactions from
// the file specified in the configs.
func loadTransactions(c *config) ([]*transaction, error) {
	csvFile, err := os.Open(c.TransactionsFile)
	if err != nil {
		return nil, err
	}
	defer csvFile.Close()

	csvReader := csv.NewReader(csvFile)

	var transactions []*transaction
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}

		// skip first row
		if record[0] == "DATE" || record[0] == "***END OF FILE***" {
			continue
		}
		nextTransaction, err := newTransactionTDA(record)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping invalid transaction due to: %v\n", err)
		}
		transactions = append(transactions, nextTransaction)
	}
	return transactions, nil
}

// groupSymbols organizes a list of transactions by their symbol.
// the function returns a map whose key's are the symbol and the
// value is a list of pointers to transactions with the same symbol.
//
// this function will ignore transactions with a blank symbol
func groupSymbols(trans []*transaction) map[string][]*transaction {
	var results = make(map[string][]*transaction)
	for i := 0; i < len(trans); i++ {
		var t *transaction = trans[i]
		trimmedSymbol := strings.TrimSpace(t.Symbol)
		// guard clause to ignore any blank symbol transactions
		if len(trimmedSymbol) == 0 {
			continue
		}

		if results[trimmedSymbol] != nil {
			// previously found symbol, append this transaction
			transactionList := results[trimmedSymbol]
			results[trimmedSymbol] = append(transactionList, t)
		} else {
			// first time encountering symbol, make a new entry
			transactionList := make([]*transaction, 1)
			transactionList[0] = t
			results[trimmedSymbol] = transactionList
		}
	}

	return results
}

// groupRelatedSymbols is used to identify related options and underlying symbols.
//
// returns a mapping of symbols to a list of related symbols
// found in the input grouping of transactions.
func groupRelatedSymbols(groupedTransactions map[string][]*transaction) (results map[string][]string) {
	results = make(map[string][]string)

	for s := range groupedTransactions {
		relatedSymbols := make([]string, 0)
		for symbol := range groupedTransactions {
			if symbol != s && strings.HasPrefix(symbol, s+" ") {
				relatedSymbols = append(relatedSymbols, symbol)
			}
		}
		if len(relatedSymbols) > 0 {
			results[s] = relatedSymbols
		}
	}

	return results
}

type CostBasis struct {
	Symbol           string         // ticker symbol
	Position         *big.Float     // total open position of this cost basis excluding related positions
	PL               *big.Float     // total profit/loss of this position excluding P/L from related positions
	EffPL            *big.Float     // the total profit/loss of this position including P/L from related positions
	Transactions     []*transaction // list of transactions for this symbol
	RelatedPositions []*CostBasis   // related positions cost basis (eg options Cost basis related to an underlying position)
}

// newEffCostBasis returns a pointer to a new struct for details about
// the effective cost basis of a symbol given the symbol and a list
// of transactions.
//
// as part of the construction, the open position amount and total P/L
// is computed.
func newCostBasis(symbol string, trans []*transaction) *CostBasis {
	e := CostBasis{
		Symbol:           symbol,
		Transactions:     trans,
		RelatedPositions: make([]*CostBasis, 0),
	}

	openPos := big.NewFloat(0.0)
	totalPl := big.NewFloat(0.0)
	for i := 0; i < len(trans); i++ {
		t := trans[i]
		if t != nil {
			openPos = openPos.Add(openPos, t.Quantity)
			totalPl = totalPl.Add(totalPl, t.Amount)
		}
	}
	e.Position = openPos
	e.PL = totalPl
	e.EffPL = totalPl.Copy(totalPl)

	return &e
}

// getAvgCost returns the average price of each open unit of the position.
// does not account for effective profit/loss from related positions
func (e *CostBasis) getAvgCost() *big.Float {
	avgCost := big.NewFloat(0.0)
	avgCost = avgCost.Quo(e.PL, e.Position)
	return avgCost
}

// getEffAvgCost returns the average price of each open unit of the position.
// does not account for effective profit/loss from related positions
func (e *CostBasis) getEffAvgCost() *big.Float {
	avgCost := big.NewFloat(0.0)
	avgCost = avgCost.Quo(e.EffPL, e.Position)
	return avgCost
}

// appendCostBasis adds a cost basis as a related cost basis to the
// receiving struct. This is primarily used to add a cost basis for
// a option contract as a related cost basis to the underlying ticker
func (e *CostBasis) appendCostBasis(cb *CostBasis) {
	e.RelatedPositions = append(e.RelatedPositions, cb)
	// recompute effective PL
	updatedEffPL := big.NewFloat(0.0)
	updatedEffPL = updatedEffPL.Copy(e.PL)
	for i := 0; i < len(e.RelatedPositions); i++ {
		rp := e.RelatedPositions[i]
		if rp == nil {
			continue
		}
		updatedEffPL = updatedEffPL.Add(updatedEffPL, rp.PL)
	}
	e.EffPL = updatedEffPL.Copy(updatedEffPL)
}

// getEffectiveCostBasis computes the effective cost basis for all symbols that currently have
// an open position.
//
// this is achieved by first computing the cost basis of the shares position from the purchase
// of the shares, then applying any profits/losses from transactions on related symbols.
//
// for example, take a position of 100 AMD shares bought at $70. initial cost basis is $7000
// if there are transactions for a covered call AMD $75c that total a profit of $100, this function
// would compute the effective cost basis of the AMD shares as being $6900.
//
// if the position of shares has been bought and sold multiple times, this function will compute the
// cost basis of the shares (before factoring in options profit/loss) as the sum of all P/L from all
// transactions for the shares.
//
// so if AMD has 3 transactions, BUY 100 shares for $7000, SELL 100 shares for $7500, BUY 100 shares
// for $7600, this function will compute the shares cost basis as (-7000 + 7500 - 7600) = 7100. This is
// done to compute an accurate picture of the amount of money made or lost on the position across multiple
// buys/sells
//
// currently, this function ignores shares positions that have been closed.
func getEffectiveCostBasis(relatedSymbols map[string][]string, groupedTransactions map[string][]*transaction) []*CostBasis {
	results := make([]*CostBasis, 0)
	visitedSymbols := make(map[string]bool)
	// first group up every symbol with related symbols
	for symbol, relatedSymbols := range relatedSymbols {
		// guard clause: skip already visited symbols
		if visitedSymbols[symbol] {
			continue
		}

		symbolTransactions := groupedTransactions[symbol]
		effCB := newCostBasis(symbol, symbolTransactions)
		visitedSymbols[symbol] = true

		for relatedSymbolIdx := 0; relatedSymbolIdx < len(relatedSymbols); relatedSymbolIdx++ {
			relatedSymbol := relatedSymbols[relatedSymbolIdx]
			if visitedSymbols[relatedSymbol] {
				continue
			}
			visitedSymbols[relatedSymbol] = true
			transactions := groupedTransactions[relatedSymbol]
			relatedEffCB := newCostBasis(relatedSymbol, transactions)
			effCB.appendCostBasis(relatedEffCB)
		}
		results = append(results, effCB)
	}

	// find any symbols that weren't included in the related symbols search and add those
	for symbol, transactions := range groupedTransactions {
		// guard clause: skip anything we already processed
		if visitedSymbols[symbol] {
			continue
		}
		effCB := newCostBasis(symbol, transactions)
		results = append(results, effCB)
	}
	return results
}

type TransactionStats struct {
	CostBasis             []*CostBasis
	ProfitablePositionPct *big.Float //percentage of cost basis' that ended up being profitable
	LargestGainPosition   *CostBasis //the cost basis with the largest profit
	LargestLossPosition   *CostBasis //the cost basis with the largest loss
}

func newTransactionStats(cb []*CostBasis) *TransactionStats {
	ts := TransactionStats{
		CostBasis: cb,
	}

	// compute profitability pct
	totalCostBasis := big.NewFloat(float64(len(cb)))
	ZERO := big.NewFloat(0.0)
	ONE := big.NewFloat(1.0)
	HUNDRED := big.NewFloat(100.0)
	profitPosPct := big.NewFloat(0.0)
	var largestGain *CostBasis
	var largestLoss *CostBasis

	for i := 0; i < len(cb); i++ {
		position := cb[i]
		if position.EffPL.Cmp(ZERO) > 0 {
			profitPosPct = profitPosPct.Add(profitPosPct, ONE)
		}
		// only count zero'd out positions towards largest gain/loss
		if position.Position.Cmp(ZERO) == 0 {
			if largestGain == nil || position.EffPL.Cmp(largestGain.EffPL) > 0 {
				largestGain = position
			}
			if (largestLoss == nil || ( position.EffPL.Cmp(ZERO) < 0 && position.EffPL.Cmp(largestLoss.EffPL) < 0)) {
				largestLoss = position
			}
		}
	}

	profitPosPct = profitPosPct.Quo(profitPosPct, totalCostBasis).Mul(profitPosPct, HUNDRED)
	ts.ProfitablePositionPct = profitPosPct
	ts.LargestGainPosition = largestGain
	ts.LargestLossPosition = largestLoss
	return &ts
}

func main() {
	configs, err := getConfigs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config.json: %v\n", err)
		fmt.Printf("Using default configurations\n")
	}

	transactions, err := loadTransactions(configs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading transactions: %v", err)
		os.Exit(1)
	}

	groupedSymbols := groupSymbols(transactions)

	relatedSymbols := groupRelatedSymbols(groupedSymbols)

	cb := getEffectiveCostBasis(relatedSymbols, groupedSymbols)
	stats := newTransactionStats(cb)
	jsonStats, err := json.Marshal(stats)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serializing output: %v", err)
		os.Exit(2)
	}

	fmt.Fprintf(os.Stdout, "%v", string(jsonStats))
}
