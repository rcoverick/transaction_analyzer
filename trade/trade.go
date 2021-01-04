package trade

import (
	"time"
	"math/big"
	"strings"
)

// transaction represents a transaction from a TD Ameritrade
// account transaction log.
type Trade struct {
	Date        time.Time
	Description string
	Quantity    *big.Float
	Symbol      string
	Price       *big.Float
	Commission  *big.Float
	Amount      *big.Float
}


// NewTradeTDA constructs a new trade struct
// from a csv row in a trade transaction log downloaded from
// TD Ameritrade.
func NewTradeTDA(r []string) (*Trade, error) {

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

	t := Trade{
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