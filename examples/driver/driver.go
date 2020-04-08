package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	"github.com/celo-org/rosetta/celo/client"
	"github.com/celo-org/rosetta/celo/contract"
	"github.com/celo-org/rosetta/celo/wrapper"
	"github.com/celo-org/rosetta/internal/config"
	"github.com/celo-org/rosetta/tracer"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/k0kubun/pp"
)

var ctx = context.Background()

var coreContracts = []string{
	"Attestations",
	"BlockchainParameters",
	"Election",
	"EpochRewards",
	"FeeCurrencyWhitelist",
	"Freezer",
	"GasPriceMinimum",
	"GoldToken",
	"Governance",
	"LockedGold",
	"Random",
	"Reserve",
	"SortedOracles",
	"StableToken",
	"TransferWhitelist",
	"Validators",
}

func main() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))

	// FetchEveryXBlocksByHash()
	// CheckHeaderHash()
	// DriverRegistryErrors()
	// DriverEpochLogs()
	// DriverEpochRewards()
	// DriverEpochRewards2()
	// DriverSubscribe()
}

func PanicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func DriverEpochRewards2() {
	cc := CeloClient()
	blockNumber := big.NewInt(241920)

	header, err := cc.Eth.HeaderByNumber(ctx, blockNumber)
	PanicOnErr(err)

	rewards, err := tracer.ComputeEpochRewards(ctx, cc, header)
	PanicOnErr(err)

	for add, value := range rewards {
		fmt.Printf("%s = %s\n", add, value)
	}

}
func DriverEpochRewards() {
	cc := CeloClient()

	blockHash := common.HexToHash("0x80ec51d5487a798a15c5e89da37b3b12598e81abaeacc9d3d68f1f721b8c50cb")
	logs, err := cc.Eth.FilterLogs(ctx, ethereum.FilterQuery{
		BlockHash: &blockHash,
	})
	PanicOnErr(err)

	addresses, _ := GetAllRegistryAddresses(cc)

	for _, log := range logs {
		if log.TxHash == blockHash {
			pp.Println(addresses[log.Address], hex.EncodeToString(log.Data))

		}
	}
}

type LogParser interface {
	TryParseLog(log types.Log) (eventName string, event interface{}, ok bool, err error)
}

func DriverEpochLogs() {
	cc := CeloClient()

	addresses, nameToAddr := GetAllRegistryAddresses(cc)
	parsers := make(map[common.Address]LogParser)

	election, err := contract.NewElection(nameToAddr["Election"], cc.Eth)
	PanicOnErr(err)
	parsers[nameToAddr["Election"]] = election

	validators, err := contract.NewValidators(nameToAddr["Validators"], cc.Eth)
	PanicOnErr(err)
	parsers[nameToAddr["Validators"]] = validators

	stableToken, err := contract.NewStableToken(nameToAddr["StableToken"], cc.Eth)
	PanicOnErr(err)
	parsers[nameToAddr["StableToken"]] = stableToken

	epochRewards, err := contract.NewEpochRewards(nameToAddr["EpochRewards"], cc.Eth)
	PanicOnErr(err)
	parsers[nameToAddr["EpochRewards"]] = epochRewards

	blockHash := common.HexToHash("0x80ec51d5487a798a15c5e89da37b3b12598e81abaeacc9d3d68f1f721b8c50cb")
	logs, err := cc.Eth.FilterLogs(ctx, ethereum.FilterQuery{
		BlockHash: &blockHash,
	})
	PanicOnErr(err)

	for _, log := range logs {
		if log.TxHash == blockHash {
			if parser, ok := parsers[log.Address]; ok {
				if eventName, data, ok, err := parser.TryParseLog(log); ok {
					PanicOnErr(err)
					pp.Println(addresses[log.Address], eventName)
					_ = data
					// jsonData, _ := json.MarshalIndent(data, "", "  ")
					// pp.Println(string(jsonData))
				}

			}
		}
	}

}

func GetAllRegistryAddresses(cc *client.CeloClient) (map[common.Address]string, map[string]common.Address) {

	registry, err := wrapper.NewRegistry(cc)
	PanicOnErr(err)

	addrToName := make(map[common.Address]string)
	nameToAddr := make(map[string]common.Address)
	for _, name := range coreContracts {
		address, _ := registry.GetAddressForString(nil, name)
		addrToName[address] = name
		nameToAddr[name] = address
	}
	return addrToName, nameToAddr
}

func DriverTransactionTransfer() {
	cc := CeloClient()

	txHash := common.HexToHash("0xc21b6442885f4375e323097d567c8a406c1589a30b984c768de536add1d7b2e1")

	transfers, err := cc.Debug.TransactionTransfers(ctx, txHash)
	PanicOnErr(err)

	pp.Print(transfers)
}

func DriverRegistryErrors() {
	cc := CeloClient()

	registry, err := wrapper.NewRegistry(cc)
	PanicOnErr(err)

	// // Find where Registry Was deployed
	// value := BinarySearch(0, 226008, func(n uint) bool {
	// 	_, err := registry.GetAddressForString(&bind.CallOpts{
	// 		BlockNumber: new(big.Int).SetUint64(uint64(n)),
	// 	}, "StableToken")
	// 	return err != wrapper.ErrRegistryNotDeployed
	// })
	// fmt.Println(value)

	// Answer is 670 it already exist, so deployed on 669

	// registry.GetUpdatesOnBlock((ctx, 669))

	// Find where Registry Was deployed
	value := BinarySearch(670, 226008, func(n uint) bool {
		_, err := registry.GetAddressForString(&bind.CallOpts{
			BlockNumber: new(big.Int).SetUint64(uint64(n)),
		}, "Governance")
		return err != client.ErrContractNotDeployed
	})
	fmt.Println(value)
}

func TxContextDriver() {
	cc := CeloClient()

	txHash := common.HexToHash("0xd6ab1c883179b677d2120c7a0d2f2a32351bd735b5c76386d13b2c23eb33ce4c")
	blockNumber := big.NewInt(222130)

	header, err := cc.Eth.HeaderByNumber(ctx, blockNumber)
	PanicOnErr(err)

	tx, _, err := cc.Eth.TransactionByHash(ctx, txHash)
	PanicOnErr(err)

	receipt, err := cc.Eth.TransactionReceipt(ctx, tx.Hash())
	PanicOnErr(err)

	txTracer := tracer.NewTxTracer(
		ctx,
		cc,
		nil,
		header,
		tx,
		receipt,
	)

	gasDetail, err := txTracer.GasDetail()
	pp.Print(gasDetail)
	pp.Print(err)
}

func CheckHeaderHash() {
	celo := CeloClient()
	lastBlock, err := celo.Eth.HeaderByNumber(context.Background(), nil)
	PanicOnErr(err)

	block, err := celo.Eth.HeaderByHash(context.Background(), lastBlock.ParentHash)
	PanicOnErr(err)

	log.Info("Block", "num", block.Number, "hash", block.Hash().Hex(), "correctHash", lastBlock.ParentHash.Hex())
}

func FetchEveryXBlocksByHash() {
	step := big.NewInt(1)
	celo, err := client.Dial(config.FornoAlfajoresUrl)
	PanicOnErr(err)

	lastBlock, err := celo.Eth.HeaderByNumber(context.Background(), nil)
	PanicOnErr(err)

	log.Info("LastBlock", "num", lastBlock.Number, "hash", lastBlock.Hash().Hex(), "parentHash", lastBlock.ParentHash.Hex())

	currBlock := lastBlock
	initial := currBlock.Number
	for {
		nextNumber := new(big.Int).Sub(currBlock.Number, step)

		logger := log.New("num", nextNumber, "distance", new(big.Int).Sub(initial, nextNumber))

		logger.Info("Fetching By Number")
		block, err := celo.Eth.HeaderAndTxnHashesByNumber(context.Background(), nextNumber)
		if err != nil {
			logger.Info("Error Fetching By Number", "err", err)
			os.Exit(1)
		}

		blockHash := block.Hash()
		logger = logger.New("hash", blockHash.Hex())
		logger.Info("Fetching By hash")

		block, err = celo.Eth.HeaderAndTxnHashesByHash(context.Background(), blockHash)
		if err != nil {
			logger.Info("Error Fetching By Hash", "err", err)
			os.Exit(1)
		}
		currBlock = &block.Header
	}

}

func FetchAllBlockByHash() {
	celo, err := client.Dial(config.FornoAlfajoresUrl)
	PanicOnErr(err)

	lastBlock, err := celo.Eth.HeaderByNumber(context.Background(), nil)
	PanicOnErr(err)

	log.Info("LastBlock", "num", lastBlock.Number, "hash", lastBlock.Hash)

	currBlock := lastBlock
	for {
		log.Info("Fetching", "num", currBlock.Number.Int64()-1, "hash", currBlock.ParentHash.Hex())
		parentBlock, err := celo.Eth.HeaderByHash(context.Background(), currBlock.ParentHash)
		if err != nil {
			log.Info("Error Fetching", "num", currBlock.Number.Int64()-1, "hash", currBlock.ParentHash.Hex(), "err", err)
			block, err := celo.Eth.HeaderAndTxnHashesByNumber(context.Background(), new(big.Int).Sub(currBlock.Number, big.NewInt(1)))
			if err != nil {
				panic(err)
			}
			MustPPHeader(block)

			os.Exit(1)
		}
		currBlock = parentBlock
	}
}

//885363
//885671

func MustPPHeader(block *ethclient.HeaderAndTxnHashes) {
	str, err := json.MarshalIndent(block, "  ", "  ")
	PanicOnErr(err)
	fmt.Println(string(str))
}

func CeloClientWithUri(uri string) *client.CeloClient {
	celo, err := client.Dial(uri)
	PanicOnErr(err)
	return celo
}

func CeloClient() *client.CeloClient {
	return CeloClientWithUri(config.FornoRC0Url)
}

func BinarySearch(low uint, right uint, test func(n uint) bool) uint {
	lowValue := test(low)
	rightValue := test(right)
	if lowValue == rightValue {
		panic("Bad Initial Values")
	}

	for low+1 != right {
		mid := (low + right) / 2
		fmt.Printf("low=%d  mid=%d  right=%d\n", low, mid, right)

		if test(mid) == lowValue {
			low = mid
		} else {
			right = mid
		}
	}
	return right
}
