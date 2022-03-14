package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strconv"

	cryptomulti "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	amino "github.com/tendermint/go-amino"
)

// round to 2 decimal places
func Round2(x float64) (r float64) {
	s := fmt.Sprintf("%.2f", x)
	r, _ = strconv.ParseFloat(s, 64)
	return
}

// Accumulate values from toSum into sum and return the total of toSum
func AccumMap(toSum, sum map[string]float64) float64 {
	var total float64
	for addr, amt := range toSum {
		if _, ok := sum[addr]; ok {
			fmt.Println("Duplicate addr, consolidating", addr)
		}
		if amt <= 0 {
			panic(fmt.Sprintf("Non positive amount for addr (%v): %v", addr, amt))
		}
		sum[addr] += amt
		total += amt
	}
	return total
}

// Sum all elements in toSum
func SumMap(toSum map[string]float64) float64 {
	var total float64
	for _, amt := range toSum {
		total += amt
	}
	return total
}

type BalanceAlloc struct {
	Amt  float64 `json:"amt"`
	Memo string  `json:"memo"`
}

// Load a JSON object of addr->amt into a map.
// Expects no duplicates!
// TODO: remove this and do everything through lists so duplicates can always be detected?
func ObjToMap(file string) map[string]BalanceAlloc {
	bz, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	m := make(map[string]BalanceAlloc)
	err = json.Unmarshal(bz, &m)
	if err != nil {
		panic(err)
	}
	return m
}

// Load a flattened list of (addr, amt) pairs into a map
// and consolidate any duplicates.
// Panics on odd length, prints duplicates.
func ListToMap(file string) map[string]float64 {
	bz, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}

	var l []interface{}
	err = json.Unmarshal(bz, &l)
	if err != nil {
		panic(err)
	}

	// list should be pairs of addr, amt
	if len(l)%2 != 0 {
		panic(fmt.Errorf("list length is odd"))
	}

	// loop through two at a time and add the amt to the entry
	// in the map for the addr
	amounts := make(map[string]float64)
	for i := 0; i < len(l); i += 2 {
		addr := l[i].(string)
		amt := l[i+1].(float64)
		if _, ok := amounts[addr]; ok {
			fmt.Println("Duplicate addr, consolidating", addr)
		}
		amounts[addr] += amt
	}
	return amounts
}

// check the address is correct for the sorted pubkey multisig
func CheckMultisigAddress(k int, pubStrings []string, addr string) {
	cdc := amino.NewCodec()
	var pubs []cryptotypes.PubKey
	for _, pubString := range pubStrings {
		// bech32 decode, then amino decode
		_, bz, err := bech32.DecodeAndConvert(pubString)
		var pubkey cryptotypes.PubKey
		err = cdc.UnmarshalBinaryBare(bz, &pubkey)
		if err != nil {
			panic(fmt.Sprintf("unmarshaling pubkey %v: %v", pubString, err))
		}
		pubs = append(pubs, pubkey)

	}

	// sort the keys
	sort.Slice(pubs, func(i, j int) bool {
		return bytes.Compare(pubs[i].Address(), pubs[j].Address()) < 0
	})

	pubKey := cryptomulti.NewLegacyAminoPubKey(k, pubs)
	pubKeyAddr := sdk.AccAddress(pubKey.Address()).String()
	if pubKeyAddr != addr {
		panic(fmt.Errorf("computed addr (%v) does not match given addr (%v)", pubKeyAddr, addr))
	}
}
