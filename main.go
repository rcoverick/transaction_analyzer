package main

import (
	"fmt"
	"os"
)

// config holds configurable values
// that can be set to define how the program runs,
// what stats are computed and how they're aggregated,
// what file(s) to read transactions from, etc..
type config struct {
	TransactionsFile string
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
func getConfigs() (*config,  error) {
	configFile, err := os.Open("config.json")
	config := newConfig()
	if err != nil {
		// use sane default configurations
		return config, nil
	} else {
		defer configFile.Close()
	}
	return config, err
}

func main() {
	configs, err := getConfigs()
	if err != nil{
		fmt.Fprintf(os.Stderr, "Error loading config.json: %v\n", err)
		fmt.Printf("Using default configurations\n")
	}
	fmt.Fprintf(os.Stdout,"Configs: %v", configs)
	
}
