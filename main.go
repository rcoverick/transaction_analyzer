package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
)

// config holds configurable values
// that can be set to define how the program runs,
// what stats are computed and how they're aggregated,
// what file(s) to read transactions from, etc..
type config struct {
	TransactionsFile string `json:"transactionsFile"`
}

type transaction struct {
	Date        string
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
func newTransactionTDA(r []string) *transaction {

	quantity, _, err := big.ParseFloat(r[3], 10, 2, big.ToNearestEven)
	if err != nil {
		quantity = big.NewFloat(0) // sane default
	}

	price, _, err := big.ParseFloat(r[5], 10, 2, big.ToNearestEven)
	if err != nil {
		price = big.NewFloat(0)
	}

	commission, _, err := big.ParseFloat(r[6], 10, 2, big.ToNearestEven)
	if err != nil {
		commission = big.NewFloat(0)
	}

	amount, _, err := big.ParseFloat(r[7], 10, 2, big.ToNearestEven)
	if err != nil {
		amount = big.NewFloat(0)
	}

	t := transaction{
		Date:        r[0],
		Description: r[2],
		Symbol:      r[4],
		Quantity:    quantity,
		Price:       price,
		Commission:  commission,
		Amount:      amount}
	return &t

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
		transactions = append(transactions, newTransactionTDA(record))
	}
	return transactions, nil
}

func main() {
	configs, err := getConfigs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config.json: %v\n", err)
		fmt.Printf("Using default configurations\n")
	}
	fmt.Fprintf(os.Stdout, "Configs: %v", configs)
	transactions, err := loadTransactions(configs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading transactions: %v", err)
		os.Exit(1)
	}
	for i := 0; i < len(transactions); i++ {
		fmt.Fprintf(os.Stdout, "%v", transactions[i])
	}
}
