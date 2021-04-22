// tda_models contains structs and parsing functions for 
// representing data from tdameritrade.
package main
import (
	"time"
	"strconv"
)

type Transaction struct {
	Date time.Time
	TransactionID string 
	Description string 
	Quantity int
	Symbol string 
	Price float32 // TODO make not a float for precision 
	Commission float32 // TODO make not a float for precision
	Amount float32 // TODO make this not a float for precision 
	RegFee float32 // TODO make this not a float for precision
}


// NewTransaction takes a key/value map from parsing the transaction csv 
// file provided by TD Ameritrade and returns a pointer to a new Transaction struct
// with the parsed values
func NewTransaction(csvRow *map[string]string) (*Transaction, error) {
	transDate, err := time.Parse("01/02/2006", (*csvRow)["DATE"])
	if err != nil {
		return nil, err 
	}

	transQty, err := strconv.ParseInt((*csvRow)["QUANTITY"],0,32)
	if err != nil {
		return nil, err 
	}
	
	transPrice, err := strconv.ParseFloat((*csvRow)["PRICE"],32) 
	if err != nil {
		return nil, err 
	}

	transCommission, err := strconv.ParseFloat((*csvRow)["COMMISSION"],32)
	if err != nil {
		return nil, err 
	}

	transAmount, err := strconv.ParseFloat((*csvRow)["AMOUNT"],32)
	if err != nil {
		return nil, err 
	}

	transRegFee, err := strconv.ParseFloat((*csvRow)["REG FEE"], 32)
	if err != nil {
		return nil, err 
	}

	t := Transaction{
		Date: transDate,
		TransactionID: (*csvRow)["TRANSACTION ID"],
		Description: (*csvRow)["DESCRIPTION"],
		Quantity: int(transQty),
		Symbol: (*csvRow)["SYMBOL"],
		Price: float32(transPrice),
		Commission: float32(transCommission),
		Amount: float32(transAmount),
		RegFee: float32(transRegFee),
	}

	return &t, nil 
}