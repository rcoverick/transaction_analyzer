package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	// "math/big"
	"os"
	"strings"
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
func loadTransactions(c *config) ([]map[string]string, error) {
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
	
	transactions := make([]map[string]string, 10)
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

		transactions = append(transactions, newRecord)
	}
	return transactions, nil
}


// groups transactions according to underlying symbol
func groupTransactionsByUnderlying(transactions *[]map[string]string) (map[string][]map[string]string) {
	
	results := make(map[string][]map[string]string)
	for i:= 0; i<len(*transactions); i++ {
		// for options, first full word will be the underlyign symbol
		transaction := (*transactions)[i]
		symbol := strings.Split(transaction["SYMBOL"], " ")[0]
		if _,exists := results[symbol]; exists {
			results[symbol] = append(results[symbol],transaction)
		} else {
			resultSlice := make([]map[string]string, 0)
			resultSlice = append(resultSlice, transaction)
			results[symbol] = resultSlice
		}
	}
	return results
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

	grouped := groupTransactionsByUnderlying(&transactions)

	json.NewEncoder(os.Stdout).Encode(grouped)
}
