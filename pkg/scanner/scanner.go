package scanner

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"

	"github.com/InjectiveLabs/sdk-go/client/common"
	explorerclient "github.com/InjectiveLabs/sdk-go/client/explorer"
	explorerPB "github.com/InjectiveLabs/sdk-go/exchange/explorer_rpc/pb"

	// Import your sub-packages
	logParser "github.com/kprimice/challenge-week/pkg/scanner/logs"
	// msgParser "github.com/kprimice/challenge-week/pkg/scanner/msg" // if you want messages
	"github.com/kprimice/challenge-week/pkg/scanner/types"
)

func RunScanner(cfg types.Config, ordersFile *os.File, tradesFile *os.File) error {
	if cfg.StartBlock > cfg.EndBlock {
		return fmt.Errorf(
			"start block %d must be <= end block %d",
			cfg.StartBlock, cfg.EndBlock,
		)
	}

	// Load network info and create an Explorer client
	network := common.LoadNetwork("mainnet", "lb")
	client, err := explorerclient.NewExplorerClient(network)
	if err != nil {
		return fmt.Errorf("failed to create explorer client: %w", err)
	}

	// Prepare CSV output
	ordersWriter := csv.NewWriter(ordersFile)
	tradesWriter := csv.NewWriter(tradesFile)
	defer ordersWriter.Flush()
	defer tradesWriter.Flush()

	ordersWriter.Write([]string{
		"OrderHash", "Block", "Action", "Price", "Quantity", "Margin", "OrderType", "SubaccountID",
	})

	tradesWriter.Write([]string{
		"OrderHash", "Block", "Action", "ExecPrice", "ExecQuantity", "ExecFee", "IsBuy", "IsLiquidation", "Pnl", "Payout", "SubaccountID",
	})

	log.Printf("Scanning from block %d up to %d...", cfg.StartBlock, cfg.EndBlock)

	// Keep track of how many log records we produce
	var totalMatches int64

	// For storing subaccount+cid -> orderHash
	// orderHashMap := make(map[string]string)

	// Adjust these as desired
	const chunkSize = uint64(100) // how many blocks per chunk
	const pageSize = int32(100)   // how many txs per fetch

	// Outer loop: chunk over block ranges
	for chunkLow := cfg.StartBlock; chunkLow <= cfg.EndBlock; chunkLow += chunkSize {
		chunkHigh := chunkLow + chunkSize - 1
		if chunkHigh > cfg.EndBlock {
			chunkHigh = cfg.EndBlock
		}

		log.Printf("Processing block chunk %d .. %d", chunkLow, chunkHigh)

		// We'll keep fetching in pages until no more Tx
		var skip uint64

		for {
			req := &explorerPB.GetTxsRequest{
				// These define the block range we want
				After:  chunkLow,  // >= chunkLow
				Before: chunkHigh, // <= chunkHigh

				// Normal pagination
				Limit: pageSize,
				Skip:  skip,
			}

			// Retry if the Explorer node is momentarily unavailable
			res, err := GetTxsWithRetry(context.Background(), client, req, 3)
			if err != nil {
				log.Printf("GetTxs error for chunk [%d..%d]: %v", chunkLow, chunkHigh, err)
				// Break this chunk and continue with the next
				break
			}
			// log number of lines returned
			// log.Printf("Number of lines returned: %d", len(res.Data))

			txs := res.Data
			if len(txs) == 0 {
				// No more txs in this block range
				break
			}

			// Process each transaction
			for _, tx := range txs {
				// log txid parsed & block
				// 1) Update orderHashMap from logs (you parse 'buy_orders', 'sell_orders', etc.)
				/*
					newHashes := logParser.BuildOrderHashMap(tx)
					for k, v := range newHashes {
						orderHashMap[k] = v
					}
				*/

				// 2) (Optional) parse messages if you want
				// msgRecords := msgParser.ParseTxMessages(tx, orderHashMap)

				// 3) Parse logs for actual events (new orders, cancels, executions, etc.)
				// log.Printf("Processing tx %s from block %d", tx.Hash, tx.BlockNumber)
				logRecords := logParser.ParseTxLogs(tx, cfg.MarketID)

				// Write all log-based records to CSV
				for _, rec := range logRecords {
					switch rec.Action {
					case "EVENT_NEW", "EVENT_CANCEL":
						ordersWriter.Write(rec.AsOrderRow())
					case "EXECUTION":
						tradesWriter.Write(rec.AsTradeRow())
					}
				}
				ordersWriter.Flush()
				tradesWriter.Flush()

				totalMatches += int64(len(logRecords))
			}

			// Increase skip by how many Tx we just processed
			skip += uint64(len(txs))

			// If we got fewer than 'pageSize' Tx, no more results remain in this chunk
			if len(txs) < int(pageSize) {
				break
			}
		}
	}

	log.Printf("Finished => found %d records from block %d up to %d.",
		totalMatches, cfg.StartBlock, cfg.EndBlock)
	return nil
}
