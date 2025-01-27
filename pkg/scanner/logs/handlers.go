package logs

import (
	"encoding/json"
	"log"
	"strings"

	explorerPB "github.com/InjectiveLabs/sdk-go/exchange/explorer_rpc/pb"

	"github.com/kprimice/challenge-week/pkg/scanner/types"
)

func ParseTxLogs(tx *explorerPB.TxData, marketID string) []types.CSVRecord {
	var logs []types.TxLog
	if err := json.Unmarshal([]byte(tx.Logs), &logs); err != nil {
		return nil
	}

	var results []types.CSVRecord
	for _, l := range logs {
		for _, e := range l.Events {
			if strings.HasPrefix(e.Type, "injective.exchange.v1beta1.") {
				switch e.Type {
				case "injective.exchange.v1beta1.EventCancelDerivativeOrder":
					results = append(results, handleEventCancel(tx, e.Attributes, marketID)...)
				case "injective.exchange.v1beta1.EventNewDerivativeOrders":
					results = append(results, handleEventNewOrders(tx, e.Attributes, marketID)...)
				case "injective.exchange.v1beta1.EventBatchDerivativeExecution":
					results = append(results, handleEventBatchDerivativeExecution(tx, e.Attributes, marketID)...)
				default:
					if strings.Contains(e.Type, "Spot") ||
						strings.Contains(e.Type, "Fail") ||
						e.Type == "injective.exchange.v1beta1.EventPerpetualMarketFundingUpdate" ||
						e.Type == "injective.exchange.v1beta1.EventSubaccountWithdraw" {
						continue
					}
					log.Printf("Unknown event type: %s for tx %s with attributes: %v\n",
						e.Type, tx.Hash, e.Attributes)
				}
			}
		}
	}
	return results
}

/*
func BuildOrderHashMap(tx *explorerPB.TxData) map[string]string {
	result := make(map[string]string)
	var logs []types.TxLog
	if err := json.Unmarshal([]byte(tx.Logs), &logs); err != nil {
		return result
	}

	for _, l := range logs {
		for _, e := range l.Events {
			if e.Type == "injective.exchange.v1beta1.EventNewDerivativeOrders" {
				for _, attr := range e.Attributes {
					if attr.Key == "buy_orders" || attr.Key == "sell_orders" {
						var orders []types.LimitOrder
						if err := json.Unmarshal([]byte(attr.Value), &orders); err == nil {
							for _, lo := range orders {
								key := lo.OrderInfo.SubaccountID + "|" + lo.OrderInfo.Cid
								result[key] = lo.OrderHash
							}
						}
					}
				}
			}
		}
	}
	return result
}
*/

func handleEventCancel(tx *explorerPB.TxData, attrs []types.EventAttribute, filterMarketID string) []types.CSVRecord {
	var records []types.CSVRecord
	var topLevelMarketID string

	// 1) Collect top-level market_id (if any)
	for _, attr := range attrs {
		if attr.Key == "market_id" {
			topLevelMarketID = strings.Trim(attr.Value, `"`)
			break
		}
	}

	if filterMarketID != "" && filterMarketID != topLevelMarketID {
		return records // empty
	}

	// 2) Parse the "limit_order" attribute
	for _, attr := range attrs {
		if attr.Key == "limit_order" {
			var lo types.LimitOrder
			if err := json.Unmarshal([]byte(attr.Value), &lo); err == nil {
				if lo.MarketId == "" {
					lo.MarketId = topLevelMarketID
				}
				rec := types.CSVRecord{
					TxHash:         tx.Hash,
					Block:          tx.BlockNumber,
					BlockTimestamp: tx.BlockTimestamp,
					Action:         "EVENT_CANCEL",
					MarketID:       lo.MarketId,
					Price:          lo.OrderInfo.Price,
					Quantity:       lo.OrderInfo.Quantity,
					OrderType:      lo.OrderType,
					SubaccountID:   lo.OrderInfo.SubaccountID,
					Margin:         lo.Margin,
					OrderHash:      lo.OrderHash,
				}
				records = append(records, rec)
			}
		}
	}
	return records
}

func handleEventNewOrders(tx *explorerPB.TxData, attrs []types.EventAttribute, filterMarketID string) []types.CSVRecord {
	var records []types.CSVRecord
	var topLevelMarketID string

	// 1) Collect top-level market_id (if any)
	for _, attr := range attrs {
		if attr.Key == "market_id" {
			topLevelMarketID = strings.Trim(attr.Value, `"`)
			break
		}
	}

	if filterMarketID != "" && filterMarketID != topLevelMarketID {
		return records // empty
	}

	// 2) Parse buy_orders / sell_orders arrays
	for _, attr := range attrs {
		if attr.Key == "buy_orders" || attr.Key == "sell_orders" {
			var orders []types.LimitOrder
			if err := json.Unmarshal([]byte(attr.Value), &orders); err != nil {
				// Could log if you want: log.Printf("Failed to unmarshal buy/sell_orders: %v", err)
				continue
			}
			for _, lo := range orders {
				if lo.MarketId == "" {
					lo.MarketId = topLevelMarketID
				}
				rec := types.CSVRecord{
					TxHash:         tx.Hash,
					Block:          tx.BlockNumber,
					BlockTimestamp: tx.BlockTimestamp,
					Action:         "EVENT_NEW",
					MarketID:       lo.MarketId,
					Price:          lo.OrderInfo.Price,
					Quantity:       lo.OrderInfo.Quantity,
					OrderType:      lo.OrderType,
					SubaccountID:   lo.OrderInfo.SubaccountID,
					Margin:         lo.Margin,
					OrderHash:      lo.OrderHash,
				}
				records = append(records, rec)
			}
		}
	}

	return records
}

func handleEventBatchDerivativeExecution(
	tx *explorerPB.TxData,
	attrs []types.EventAttribute,
	filterMarketID string,
) []types.CSVRecord {
	var records []types.CSVRecord

	var marketID string
	var isBuy bool
	var isLiquidation bool
	var tradesRaw string
	var execType string

	for _, attr := range attrs {
		switch attr.Key {
		case "market_id":
			marketID = attr.Value
		case "is_buy":
			// "true" or "false" as string
			isBuy = (attr.Value == "true")
		case "executionType":
			execType = attr.Value // e.g. "market", "limitFill", "limitMatchRestingOrder", "limitMatchNewOrder"
			log.Println("Execution type: ", execType)
		case "is_liquidation":
			isLiquidation = (attr.Value == "true")
		case "trades":
			tradesRaw = attr.Value
		default:
			log.Printf("Unknown attribute key: %s\n", attr.Key)
		}
	}

	if filterMarketID != "" && filterMarketID != marketID {
		return records // empty
	}

	// 2) If no trades attribute, nothing to parse
	if tradesRaw == "" {
		return records
	}

	// 3) Decode the JSON array of trades
	var trades []types.BatchDerivativeTrade
	if err := json.Unmarshal([]byte(tradesRaw), &trades); err != nil {
		log.Printf("Failed to unmarshal trades in EventBatchDerivativeExecution: %v\n", err)
		return records
	}

	// 4) Create a CSV record for each trade
	for _, t := range trades {
		rec := types.CSVRecord{
			TxHash:         tx.Hash,
			Block:          tx.BlockNumber,
			BlockTimestamp: tx.BlockTimestamp,
			Action:         "EXECUTION",
			MarketID:       marketID,
			SubaccountID:   t.SubaccountId,
			OrderHash:      t.OrderHash,

			// Fill in the 'Exec' fields
			ExecPrice:    t.PositionDelta.ExecutionPrice,
			ExecQuantity: t.PositionDelta.ExecutionQuantity,
			ExecFee:      t.Fee,

			// Our new fields
			IsBuy:         isBuy,
			IsLiquidation: isLiquidation,
			Pnl:           t.Pnl,
			Payout:        t.Payout,
		}
		records = append(records, rec)
	}

	return records
}
