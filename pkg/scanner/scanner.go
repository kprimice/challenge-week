package scanner

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/InjectiveLabs/sdk-go/client/common"
	exchangeclient "github.com/InjectiveLabs/sdk-go/client/exchange"
	explorerclient "github.com/InjectiveLabs/sdk-go/client/explorer"
	derivativeExchangePB "github.com/InjectiveLabs/sdk-go/exchange/derivative_exchange_rpc/pb"
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
		"OrderHash", "Block", "Action", "Price", "Quantity", "Margin", "OrderType", "SubaccountID", "MarketID",
	})

	tradesWriter.Write([]string{
		"OrderHash", "Block", "Action", "ExecPrice", "ExecQuantity", "ExecFee", "IsBuy", "IsLiquidation", "Pnl", "Payout", "SubaccountID", "MarketID",
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

// DerivativeTradesConfig configures how we fetch trades
type DerivativeTradesConfig struct {
	MarketID   string
	PageSize   uint64
	StartBlock uint64 // user-supplied
	EndBlock   uint64 // user-supplied
	// We'll dynamically resolve to startTimeMs, endTimeMs
}

func fetchBlockTimestampMs(ctx context.Context, explorerCl explorerclient.ExplorerClient, blockNum uint64) (int64, error) {
	blockStr := strconv.FormatUint(blockNum, 10)
	blockRes, err := explorerCl.GetBlock(ctx, blockStr)
	if err != nil {
		return 0, err
	}
	// blockRes.Data.Timestamp might be e.g. "2025-01-18 15:36:50.931 +0000 UTC"
	// parse it with time.Parse
	t, err := time.Parse("2006-01-02 15:04:05.999 -0700 MST", blockRes.Data.Timestamp)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

func RunDerivativeTrades(cfg DerivativeTradesConfig, out io.Writer) error {
	// 1) Create exchange client for the derivative trades
	network := common.LoadNetwork("mainnet", "lb")
	exchClient, err := exchangeclient.NewExchangeClient(network)
	if err != nil {
		return fmt.Errorf("failed to create derivative exchange client: %w", err)
	}

	// 2) If user specified start/end block, get timestamps
	var startTimeMs, endTimeMs int64
	if cfg.StartBlock != 0 || cfg.EndBlock != 0 {
		// Create explorer client to fetch block timestamps
		explorerCl, err := explorerclient.NewExplorerClient(network)
		if err != nil {
			return fmt.Errorf("failed to create explorer client for block timestamps: %w", err)
		}
		ctx := context.Background()

		if cfg.StartBlock != 0 {
			ts, err := fetchBlockTimestampMs(ctx, explorerCl, cfg.StartBlock)
			if err != nil {
				log.Printf("Warning: can't fetch start block %d => ignoring time filter", cfg.StartBlock)
			} else {
				startTimeMs = ts
			}
		}
		if cfg.EndBlock != 0 {
			ts, err := fetchBlockTimestampMs(ctx, explorerCl, cfg.EndBlock)
			if err != nil {
				log.Printf("Warning: can't fetch end block %d => ignoring time filter", cfg.EndBlock)
			} else {
				endTimeMs = ts
			}
		}
	}

	// 3) Prepare CSV
	writer := csv.NewWriter(out)
	defer writer.Flush()

	// 4) Write CSV header
	writer.Write([]string{
		"TradeId",
		"MarketId",
		"OrderHash",
		"SubaccountId",
		"ExecPrice",
		"ExecQuantity",
		"TradeDirection",
		"Fee",
		"IsLiquidation",
		"ExecutionSide",
		"Timestamp", // so we can see actual time
	})

	pageSize := cfg.PageSize
	if pageSize == 0 {
		pageSize = 100
	}

	log.Printf("Starting derivative trades download (market=%s, startBlock=%d, endBlock=%d, limit=%d)...",
		cfg.MarketID, cfg.StartBlock, cfg.EndBlock, pageSize,
	)

	var skip uint64
	var totalTrades uint64
	ctx2 := context.Background()

	for {
		req := &derivativeExchangePB.TradesV2Request{
			Skip:      skip,
			Limit:     int32(pageSize),
			MarketIds: []string{},
		}
		if cfg.MarketID != "" {
			req.MarketIds = []string{cfg.MarketID}
		}

		// If we have startTimeMs != 0, endTimeMs != 0, set them
		if startTimeMs > 0 {
			req.StartTime = startTimeMs
		}
		if endTimeMs > 0 {
			req.EndTime = endTimeMs
		}

		// 5) Call the endpoint
		res, err := exchClient.GetDerivativeTradesV2(ctx2, req)
		if err != nil {
			log.Printf("GetDerivativeTradesV2 error at skip=%d => %v", skip, err)
			time.Sleep(2 * time.Second)
			continue
		}

		trades := res.Trades
		if len(trades) == 0 {
			log.Printf("No more derivative trades => done. skip=%d totalTrades=%d", skip, totalTrades)
			break
		}

		// 6) Write them to CSV
		for _, t := range trades {
			time.Sleep(200 * time.Millisecond)
			pd := t.PositionDelta
			if pd == nil {
				pd = &derivativeExchangePB.PositionDelta{}
			}
			// Convert orderHash from hex => base64
			// 1) Trim leading "0x"
			orderHashHex := strings.TrimPrefix(t.OrderHash, "0x")
			// 2) Decode hex => raw bytes
			raw, err := hex.DecodeString(orderHashHex)
			if err != nil {
				// If we can't decode, fallback to original
				raw = []byte(t.OrderHash)
			}
			orderHashB64 := base64.StdEncoding.EncodeToString(raw)

			record := []string{
				t.TradeId,
				t.MarketId,
				orderHashB64,
				t.SubaccountId,
				pd.ExecutionPrice,
				pd.ExecutionQuantity,
				pd.TradeDirection, // "buy" / "sell"
				t.Fee,
				strconv.FormatBool(t.IsLiquidation),
				t.ExecutionSide, // "maker"/"taker"
			}
			writer.Write(record)
		}
		writer.Flush()

		fetched := uint64(len(trades))
		totalTrades += fetched
		skip += fetched

		log.Printf("Fetched %d trades this page, total=%d, new skip=%d", fetched, totalTrades, skip)

		// If we got fewer trades than 'limit', we assume end
		if fetched < pageSize {
			log.Printf("We got %d trades, fewer than limit=%d => done", fetched, pageSize)
			break
		}
	}

	log.Printf("Done fetching derivative trades. total=%d", totalTrades)
	return nil
}
