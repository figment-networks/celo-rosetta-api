/*
 * Rosetta
 *
 * A standard for blockchain interaction
 *
 * API version: 1.2.3
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package api

import (
	"context"
	"math/big"

	"github.com/celo-org/rosetta/analyzer"
	"github.com/celo-org/rosetta/celo"
	"github.com/celo-org/rosetta/celo/client"
	"github.com/celo-org/rosetta/db"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// BlockApiService is a service that implents the logic for the BlockApiServicer
// This service should implement the business logic for every endpoint for the BlockApi API.
// Include any external packages or services that will be required by this service.
type BlockApiService struct {
	celoClient  *client.CeloClient
	db          db.RosettaDBReader
	chainParams *celo.ChainParameters
}

// NewBlockApiService creates a default api service
func NewBlockApiService(celoClient *client.CeloClient, db db.RosettaDBReader, cp *celo.ChainParameters) BlockApiServicer {
	return &BlockApiService{
		celoClient:  celoClient,
		db:          db,
		chainParams: cp,
	}
}

func (b *BlockApiService) BlockHeader(ctx context.Context, blockIdentifier PartialBlockIdentifier) (*ethclient.HeaderAndTxnHashes, error) {
	var err error
	var blockHeader *ethclient.HeaderAndTxnHashes

	if blockIdentifier.Hash != nil {
		hash := common.HexToHash(*blockIdentifier.Hash)
		blockHeader, err = b.celoClient.Eth.HeaderAndTxnHashesByHash(ctx, hash)
		if err != nil {
			err = client.WrapRpcError(err)
			return nil, ErrCantFetchBlockHeader(err)
		}

		// If both were specified check the result matches
		if blockIdentifier.Index != nil && blockHeader.Number.Cmp(big.NewInt(*blockIdentifier.Index)) != 0 {
			return nil, ErrCantFetchBlockHeader(ErrBadBlockIdentifier)
		}

	} else if blockIdentifier.Index != nil {
		blockHeader, err = b.celoClient.Eth.HeaderAndTxnHashesByNumber(ctx, big.NewInt(*blockIdentifier.Index))
		if err != nil {
			err = client.WrapRpcError(err)
			return nil, ErrCantFetchBlockHeader(err)
		}
	} else {
		blockHeader, err = b.celoClient.Eth.HeaderAndTxnHashesByNumber(ctx, nil)
		if err != nil {
			err = client.WrapRpcError(err)
			return nil, ErrCantFetchBlockHeader(err)
		}
	}

	return blockHeader, nil

}

// Block - Get a Block
func (b *BlockApiService) Block(ctx context.Context, request BlockRequest) (interface{}, error) {

	err := ValidateNetworkId(&request.NetworkIdentifier, b.chainParams)
	if err != nil {
		return nil, err
	}

	blockHeader, err := b.BlockHeader(ctx, request.BlockIdentifier)
	if err != nil {
		return nil, err
	}

	transactions := MapTxHashesToTransaction(blockHeader.Transactions)

	// If it's the last block of the Epoch, add a transaction for the block Finalize()
	if b.chainParams.IsLastBlockOfEpoch(blockHeader.Number.Uint64()) {
		transactions = append(transactions, TransactionIdentifier{Hash: blockHeader.Hash().Hex()})
	}

	return &BlockResponse{
		Block: Block{
			BlockIdentifier:       *HeaderToBlockIdentifier(&blockHeader.Header),
			ParentBlockIdentifier: *HeaderToParentBlockIdentifier(&blockHeader.Header),
			Timestamp:             int64(blockHeader.Time), // TODO unsafe casting from uint to int 64
		},
		OtherTransactions: transactions,
	}, nil

}

// BlockTransaction - Get a Block Transaction
func (s *BlockApiService) BlockTransaction(ctx context.Context, request BlockTransactionRequest) (interface{}, error) {

	err := ValidateNetworkId(&request.NetworkIdentifier, s.chainParams)
	if err != nil {
		return nil, err
	}

	blockHeader, err := s.BlockHeader(ctx, FullToPartialBlockIdentifier(request.BlockIdentifier))
	if err != nil {
		return nil, err
	}

	txHash := common.HexToHash(request.TransactionIdentifier.Hash)

	var operations []Operation
	// Check If it's block transaction (imaginary transaction)
	if s.chainParams.IsLastBlockOfEpoch(blockHeader.Number.Uint64()) && txHash == blockHeader.Hash() {
		rewards, err := analyzer.ComputeEpochRewards(ctx, s.celoClient, s.db, &blockHeader.Header)
		if err != nil {
			return nil, err
		}
		operations = OperationsFromAnalyzer(rewards, 0)
	} else {
		// Normal transaction

		if !HeaderContainsTx(blockHeader, txHash) {
			return nil, ErrMissingTxInBlock
		}

		tx, _, err := s.celoClient.Eth.TransactionByHash(ctx, txHash)
		if err != nil {
			return nil, ErrRpcError("TransactionByHash", err)
		}

		receipt, err := s.celoClient.Eth.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			return nil, ErrRpcError("TransactionReceipt", err)
		}

		tracer := analyzer.NewTracer(ctx, s.celoClient, s.db)

		ops, err := tracer.TraceTransaction(&blockHeader.Header, tx, receipt)
		if err != nil {
			return nil, err
		}

		for _, aop := range ops {
			transferOps := OperationsFromAnalyzer(&aop, int64(len(operations)))
			operations = append(operations, transferOps...)
		}
	}

	return &BlockTransactionResponse{
		Transaction: Transaction{
			TransactionIdentifier: TransactionIdentifier{Hash: txHash.Hex()},
			Operations:            operations,
		},
	}, nil
}
