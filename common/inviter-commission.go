package common

import (
	"sync"
)

var inviterCommissionRates = map[int]float64{}
var inviterCommissionRatesMutex sync.RWMutex

func InviterCommissionRates2JSONString() string {
	inviterCommissionRatesMutex.RLock()
	defer inviterCommissionRatesMutex.RUnlock()
	jsonBytes, err := Marshal(inviterCommissionRates)
	if err != nil {
		SysError("error marshalling inviter commission rates: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateInviterCommissionRatesByJSONString(jsonStr string) error {
	inviterCommissionRatesMutex.Lock()
	defer inviterCommissionRatesMutex.Unlock()
	inviterCommissionRates = make(map[int]float64)
	return Unmarshal([]byte(jsonStr), &inviterCommissionRates)
}

func GetInviterCommissionRate(orderNumber int) (float64, bool) {
	inviterCommissionRatesMutex.RLock()
	defer inviterCommissionRatesMutex.RUnlock()
	rate, ok := inviterCommissionRates[orderNumber]
	return rate, ok
}

func HasInviterCommissionRates() bool {
	inviterCommissionRatesMutex.RLock()
	defer inviterCommissionRatesMutex.RUnlock()
	return len(inviterCommissionRates) > 0
}
