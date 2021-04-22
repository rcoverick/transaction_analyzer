// projections contains structs that represent projections 
// derived from transactions.
package projections 

import (
	"github.com/rcoverick/stonks/models"
)

type TradeVolume struct {
	TotalTrades map[string]int 
}

// NewTradeVolume initializes a new TradeVolume projection
func NewTradeVolume()(*TradeVolume) {
	t := TradeVolume{}
	t.TotalTrades = make(map[string]int)
	return &t
}

// CountTransactionCommand accepts a transaction and updates the state 
// of the TradeVolume to reflect the new transaction. 
func (tv *TradeVolume) CountTransactionCommand(t *models.Transaction) {
	symbol := t.GetUnderlyingSymbol()
	if tradeCount, exists := tv.TotalTrades[symbol]; exists {
		tv.TotalTrades[symbol] = tradeCount + 1;
	} else {
		tv.TotalTrades[symbol] = 1
	}

}