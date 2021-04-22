package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"github.com/rcoverick/stonks/models"
	"github.com/rcoverick/stonks/models/projections"
	"os"
)

// config holds configurable values
// that can be set to define how the program runs,
// what stats are computed and how they're aggregated,
// what file(s) to read transactions from, etc..
type config struct {
	TransactionsFile string `json:"transactionsFile"`
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
// the file specified in the configs as maps 
func loadTransactions(c *config) ([]*models.Transaction, error) {
	csvFile, err := os.Open(c.TransactionsFile)
	if err != nil {
		return nil, err
	}
	defer csvFile.Close()

	csvReader := csv.NewReader(csvFile)

	rawTransactions, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	
	transactions := make([]*models.Transaction, 0)
	// convert each row to a key value map 
	headerRow := rawTransactions[0]
	for i:=1; i< len(rawTransactions); i++ {
		rawRecord := rawTransactions[i]
		newRecord := make(map[string]string)
		for j:=0;j<len(headerRow);j++{
			fieldName := headerRow[j]
			fieldValue := rawRecord[j]
			newRecord[fieldName] = fieldValue
		}
		transaction, err := models.NewTransaction(&newRecord)
		if err != nil {
			fmt.Errorf("Invalid transaction: %v",err)
			continue
		}

		transactions = append(transactions, transaction)
	}
	return transactions, nil
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

	tradeVolumes := projections.NewTradeVolume() 
	for t:= 0; t<len(transactions); t++ {
		transaction := transactions[t]
		tradeVolumes.CountTransactionCommand(transaction) 
	}
	json.NewEncoder(os.Stdout).Encode(tradeVolumes)
}
