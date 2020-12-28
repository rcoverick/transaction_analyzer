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
			transactionList = append(transactionList, t)
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

type effCostBasis struct {
	Symbol        string
	TotalPosition *big.Float // total open position
	TotalPL       *big.Float
	Transactions  []*transaction
}

// newEffCostBasis returns a pointer to a new struct for details about
// the effective cost basis of a symbol given the symbol and a list
// of transactions.
//
// as part of the construction, the open position amount and total P/L
// is computed.
func newEffCostBasis(symbol string, trans []*transaction) *effCostBasis {
	e := effCostBasis{
		Symbol:       symbol,
		Transactions: trans,
	}

	openPos := big.NewFloat(0.0)
	totalPL := big.NewFloat(0.0)
	for i := 0; i < len(trans); i++ {
		t := trans[i]
		if t != nil {
			openPos = openPos.Add(openPos, t.Quantity)
			totalPL = totalPL.Add(totalPL, t.Amount)
		}
	}
	e.TotalPosition = openPos
	e.TotalPL = totalPL

	return &e
}

// getAvgCost returns the average price of each open unit of the position.
func (e *effCostBasis) getAvgCost() *big.Float {
	avgCost := big.NewFloat(0.0)
	avgCost = avgCost.Quo(e.TotalPL, e.TotalPosition)
	return avgCost
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
func getEffectiveCostBasis(relatedSymbols map[string][]string, groupedTransactions map[string][]*transaction) {
	// TODO return some kind of struct or list of structs representing effective cost basis'
	// TODO make this compute effective cost basis on open and on closed positions
	fmt.Println("Computing effective cost basis of open positions")
	for symbol, relatedSymbols := range relatedSymbols {
		symbolTransactions := groupedTransactions[symbol]
		effCB := newEffCostBasis(symbol, symbolTransactions)
		if effCB.TotalPosition.Cmp(big.NewFloat(0.0)) == 0 {
			continue // position is closed
		}
		fmt.Fprintf(os.Stdout, "%v open position: %v\n", effCB.Symbol, effCB.TotalPosition)
		fmt.Fprintf(os.Stdout, "\tcost basis of open position: %v\n", effCB.TotalPL)
		fmt.Fprintf(os.Stdout, "\tavg share price of open position before computing options P/L: %v\n", effCB.getAvgCost())

		// compute total P/L from related tickers
		totalRelatedPL := big.NewFloat(0.0)
		for relatedSymbolIdx := 0; relatedSymbolIdx < len(relatedSymbols); relatedSymbolIdx++ {
			relatedSymbol := relatedSymbols[relatedSymbolIdx]
			transactions := groupedTransactions[relatedSymbol]
			relatedEffCB := newEffCostBasis(relatedSymbol, transactions)
			fmt.Fprintf(os.Stdout, "\tP/L from %v: %v\n", relatedEffCB.Symbol, relatedEffCB.TotalPL)
			totalRelatedPL = totalRelatedPL.Add(totalRelatedPL, relatedEffCB.TotalPL)
		}
		fmt.Fprintf(os.Stdout, "\ttotal P/L from related positions: %v\n", totalRelatedPL)
		effCostBasis := big.NewFloat(0.0)
		effCostBasis = effCostBasis.Add(effCB.TotalPL, totalRelatedPL)
		effAvgSharePrice := big.NewFloat(0.0)
		effAvgSharePrice = effAvgSharePrice.Quo(effCostBasis, effCB.TotalPosition)
		fmt.Fprintf(os.Stdout, "\tcost basis of open position including related positions P/L: %v\n", effCostBasis)
		fmt.Fprintf(os.Stdout, "\tavg share price of open position after computing options P/L: %v\n", effAvgSharePrice.Text('f', 2))
	}
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

	fmt.Fprintf(os.Stdout, "Symbol : total transactions\n")

	for k, t := range groupedSymbols {
		fmt.Fprintf(os.Stdout, "\t%v : %v\n", k, len(t))
	}

	relatedSymbols := groupRelatedSymbols(groupedSymbols)

	fmt.Fprintf(os.Stdout, "Related Symbols\n")
	for symbol, relatedSymbols := range relatedSymbols {
		fmt.Fprintf(os.Stdout, "%v:\n", symbol)
		for r := 0; r < len(relatedSymbols); r++ {
			fmt.Fprintf(os.Stdout, "\t%v\n", relatedSymbols[r])
		}
	}

	getEffectiveCostBasis(relatedSymbols, groupedSymbols)
}
