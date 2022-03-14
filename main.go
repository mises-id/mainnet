package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"path"
	"sort"
	"strings"
	"time"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/types/address"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/mises-id/mainnet/pkg"

	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
)

const (
	// processed contributors files
	singlesigJSON = "accounts/singlesig.json"
	multisigJSON  = "accounts/multisig.json"

	genesisTemplate = "params/genesis_template.json"
	genTxPath       = "gentx"
	genesisFile     = "genesis.json"

	umisDenomination    = "umis"
	misGenesisTotal     = 100000000
	addressGenesisTotal = 11

	timeGenesisString = "2022-03-21 23:00:00 -0000 UTC"
)

// constants but can't use `const`
var (
	timeGenesis time.Time

	// vesting times
	timeGenesisTwoMonths time.Time
	timeGenesisOneYear   time.Time
	timeGenesisTwoYears  time.Time
)

const (
	AccountAddressPrefix = "mises"
)

var (
	AccountPubKeyPrefix    = AccountAddressPrefix + "pub"
	ValidatorAddressPrefix = AccountAddressPrefix + "valoper"
	ValidatorPubKeyPrefix  = AccountAddressPrefix + "valoperpub"
	ConsNodeAddressPrefix  = AccountAddressPrefix + "valcons"
	ConsNodePubKeyPrefix   = AccountAddressPrefix + "valconspub"
)

var config *sdk.Config = nil

func SetConfig() {
	if config == nil {
		config = sdk.GetConfig()
		config.SetBech32PrefixForAccount(AccountAddressPrefix, AccountPubKeyPrefix)
		config.SetBech32PrefixForValidator(ValidatorAddressPrefix, ValidatorPubKeyPrefix)
		config.SetBech32PrefixForConsensusNode(ConsNodeAddressPrefix, ConsNodePubKeyPrefix)
		config.Seal()
	}

}

// initialize the times!
func init() {
	var err error
	timeLayoutString := "2006-01-02 15:04:05 -0700 MST"
	timeGenesis, err = time.Parse(timeLayoutString, timeGenesisString)
	if err != nil {
		panic(err)
	}
	timeGenesisTwoMonths = timeGenesis.AddDate(0, 2, 0)
	timeGenesisOneYear = timeGenesis.AddDate(1, 0, 0)
	timeGenesisTwoYears = timeGenesis.AddDate(2, 0, 0)

	SetConfig()
}

// max precision on amt is two decimals ("centi-atoms")
func misToUMisInt(amt float64) sdk.Int {
	// amt is specified to 2 decimals ("centi-atoms").
	// multiply by 100 to get the number of centi-atoms
	// and round to int64.
	// Multiply by remaining to get uAtoms.
	var precision float64 = 100
	var remaining int64 = 10000

	cmis := int64(math.Round(amt * precision))
	uMis := cmis * remaining
	return sdk.NewInt(uMis)
}

// convert atoms with two decimal precision to coins
func newCoins(amt float64) sdk.Coins {
	uMis := misToUMisInt(amt)
	return sdk.Coins{
		sdk.Coin{
			Denom:  umisDenomination,
			Amount: uMis,
		},
	}
}

func main() {
	// for each path, accumulate the contributors file.
	// icf addresses are in bech32, fundraiser are in hex
	contribs := make(map[string]float64)
	{
		accumulateBechContributors(singlesigJSON, contribs)
	}

	// construct the genesis accounts :)
	genesisAccounts := makeGenesisAccounts(contribs)

	// check totals
	checkTotals(genesisAccounts)

	fmt.Println("-----------")
	fmt.Println("TOTAL addrs", len(genesisAccounts))
	fmt.Println("TOTAL Mis", misGenesisTotal)

	// load gentxs
	fs, err := ioutil.ReadDir(genTxPath)
	if err != nil {
		panic(err)
	}

	var genTxs []json.RawMessage
	for _, f := range fs {
		name := f.Name()
		if name == "README.md" {
			continue
		}
		bz, err := ioutil.ReadFile(path.Join(genTxPath, name))
		if err != nil {
			panic(err)
		}
		genTxs = append(genTxs, json.RawMessage(bz))
	}

	fmt.Println("-----------")
	fmt.Println("TOTAL gen txs", len(genTxs))

	err = makeGenesisDoc(genesisAccounts, genTxs, genesisFile)
	if err != nil {
		panic(err)
	}
}

func fromBech32(addr string) sdk.AccAddress {
	bech32PrefixAccAddr := "mises"
	bz, err := sdk.GetFromBech32(addr, bech32PrefixAccAddr)
	if err != nil {
		panic(err)
	}
	if len(bz) > address.MaxAddrLen {
		panic("Incorrect address length")
	}
	return sdk.AccAddress(bz)
}

// load a map of hex addresses and convert them to bech32
func accumulateHexContributors(fileName string, contribs map[string]float64) error {
	allocations := pkg.ObjToMap(fileName)

	for addr, amt := range allocations {
		bech32Addr, err := sdk.AccAddressFromHex(addr)
		if err != nil {
			return err
		}
		addr = bech32Addr.String()

		if _, ok := contribs[addr]; ok {
			fmt.Println("Duplicate addr", addr)
		}
		contribs[addr] += amt.Amt
	}
	return nil
}

func accumulateBechContributors(fileName string, contribs map[string]float64) error {
	allocations := pkg.ObjToMap(fileName)

	for addr, amt := range allocations {
		if _, ok := contribs[addr]; ok {
			fmt.Println("Duplicate addr", addr)
		}
		contribs[addr] += amt.Amt
	}
	return nil
}

//----------------------------------------------------------
// AiB Data

type Account struct {
	Address string  `json:"addr"`
	Amount  float64 `json:"amount"`
	Lock    string  `json:"lock"`
}

type MultisigAccount struct {
	Address   string   `json:"addr"`
	Threshold int      `json:"threshold"`
	Pubs      []string `json:"pubs"`
	Amount    float64  `json:"amount"`
}

// load the aib atoms and ensure there are no duplicates with the contribs
func aibAtoms(employeesFile, multisigFile string, contribs map[string]float64) (employees []Account, multisigAcc MultisigAccount) {
	bz, err := ioutil.ReadFile(employeesFile)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bz, &employees)
	if err != nil {
		panic(err)
	}

	bz, err = ioutil.ReadFile(multisigFile)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bz, &multisigAcc)
	if err != nil {
		panic(err)
	}

	for _, acc := range employees {
		if _, ok := contribs[acc.Address]; ok {
			fmt.Println("AiB Addr Duplicate", acc.Address)
		}
	}
	return
}

type AccountWithBalance struct {
	base     authtypes.GenesisAccount
	balances banktypes.Balance
}

//---------------------------------------------------------------
// gaia accounts and genesis doc

// compose the gaia genesis accounts from the inputs,
// check total and for duplicates,
// sort by address
func makeGenesisAccounts(
	contribs map[string]float64) []AccountWithBalance {

	var genesisAccounts []AccountWithBalance
	{
		// public, private, and icf contribs
		for addrStr, amt := range contribs {
			addr, err := sdk.AccAddressFromBech32(addrStr)
			if err != nil {
				panic(err)
			}

			coins, err := sdk.ParseCoinsNormalized(fmt.Sprintf("%fumis", amt*1000000))
			if err != nil {
				panic(fmt.Errorf("failed to parse coins: %w", err))
			}

			balances := banktypes.Balance{Address: addr.String(), Coins: coins.Sort()}
			baseAccount := authtypes.NewBaseAccount(addr, nil, 0, 0)
			genesisAccounts = append(genesisAccounts, AccountWithBalance{baseAccount, balances})
		}

	}

	// sort the accounts
	sort.SliceStable(genesisAccounts, func(i, j int) bool {
		return strings.Compare(
			genesisAccounts[i].base.String(),
			genesisAccounts[j].base.String(),
		) < 0
	})

	return genesisAccounts
}

// check total atoms and no duplicates
func checkTotals(genesisAccounts []AccountWithBalance) {
	// check uAtom total
	uMisTotal := sdk.NewInt(0)
	for _, account := range genesisAccounts {
		uMisTotal = uMisTotal.Add(account.balances.Coins.AmountOf("umis"))
	}
	if !uMisTotal.Equal(misToUMisInt(misGenesisTotal)) {
		panicStr := fmt.Sprintf("expected %s umis, got %s umis allocated in genesis", misToUMisInt(misGenesisTotal), uMisTotal.String())
		panic(panicStr)
	}
	if len(genesisAccounts) != addressGenesisTotal {
		panicStr := fmt.Sprintf("expected %d addresses, got %d addresses allocated in genesis", addressGenesisTotal, len(genesisAccounts))
		panic(panicStr)
	}

	// ensure no duplicates
	checkdupls := make(map[string]struct{})
	for _, acc := range genesisAccounts {
		if _, ok := checkdupls[acc.base.String()]; ok {
			panic(fmt.Sprintf("Got duplicate: %v", acc.base.String()))
		}
		checkdupls[acc.base.String()] = struct{}{}
	}
	if len(checkdupls) != len(genesisAccounts) {
		panic("length mismatch!")
	}
}

// json marshal the initial app state (accounts and gentx) and add them to the template
func makeGenesisDoc(genesisAccounts []AccountWithBalance, genTxs []json.RawMessage, genFile string) error {

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	authtypes.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	stakingtypes.RegisterInterfaces(interfaceRegistry)

	cdc := codec.NewProtoCodec(interfaceRegistry)

	TxConfig := tx.NewTxConfig(cdc, tx.DefaultSignModes)

	// read the template with the params
	appGenesisState, genDoc, err := genutiltypes.GenesisStateFromGenFile("params/genesis_template.json")
	if err != nil {
		return fmt.Errorf(
			"failed to unmarshal genesis state: %w", err)
	}

	// set genesis time
	genDoc.GenesisTime = timeGenesis

	authGenState := authtypes.GetGenesisStateFromAppState(cdc, appGenesisState)

	accs, err := authtypes.UnpackAccounts(authGenState.Accounts)
	if err != nil {
		return fmt.Errorf("failed to get accounts from any: %w", err)
	}

	bankGenState := banktypes.GetGenesisStateFromAppState(cdc, appGenesisState)

	for _, acc := range genesisAccounts {

		addr := acc.base.GetAddress()
		if accs.Contains(addr) {
			return fmt.Errorf("cannot add account at existing address %s", addr)
		}

		// Add the new account to the set of genesis accounts and sanitize the
		// accounts afterwards.
		accs = append(accs, acc.base)

		bankGenState.Balances = append(bankGenState.Balances, acc.balances)
		bankGenState.Balances = banktypes.SanitizeGenesisBalances(bankGenState.Balances)
		bankGenState.Supply = bankGenState.Supply.Add(acc.balances.Coins...)
	}

	accs = authtypes.SanitizeGenesisAccounts(accs)

	genAccs, err := authtypes.PackAccounts(accs)
	if err != nil {
		return fmt.Errorf("failed to convert accounts into any's: %w", err)
	}
	authGenState.Accounts = genAccs

	authGenStateBz, err := cdc.MarshalJSON(&authGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal auth genesis state: %w", err)
	}

	appGenesisState[authtypes.ModuleName] = authGenStateBz

	bankGenStateBz, err := cdc.MarshalJSON(bankGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal bank genesis state: %w", err)
	}

	appGenesisState[banktypes.ModuleName] = bankGenStateBz

	appState, err := json.MarshalIndent(appGenesisState, "", "  ")
	if err != nil {
		return err
	}
	genDoc.AppState = appState

	appGenTxs, _, err := genutil.CollectTxs(
		cdc, TxConfig.TxJSONDecoder(), "test", "gentx", *genDoc, banktypes.GenesisBalancesIterator{},
	)
	if err != nil {
		return err
	}

	appGenesisState, err = genutil.SetGenTxsInAppGenesisState(cdc, TxConfig.TxJSONEncoder(), appGenesisState, appGenTxs)
	if err != nil {
		return err
	}

	appState, err = json.MarshalIndent(appGenesisState, "", "  ")
	if err != nil {
		return err
	}
	genDoc.AppState = appState
	return genutil.ExportGenesisFile(genDoc, genFile)
}
