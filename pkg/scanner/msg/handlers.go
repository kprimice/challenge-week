package msg

import (
	"encoding/json"
	"log"

	explorerPB "github.com/InjectiveLabs/sdk-go/exchange/explorer_rpc/pb"
	"github.com/kprimice/challenge-week/pkg/scanner/types"
)

func ParseTxMessages(tx *explorerPB.TxData, hashMap map[string]string) []types.CSVRecord {
	var results []types.CSVRecord

	var rawMsgs []json.RawMessage
	if err := json.Unmarshal([]byte(tx.Messages), &rawMsgs); err != nil {
		log.Printf("Skipping tx %s: can't unmarshal messages => %v", tx.Hash, err)
		return results
	}

	for _, rm := range rawMsgs {
		records := handleMessage(rm, tx, hashMap)
		results = append(results, records...)
	}
	return results
}

func handleMessage(rawMsg json.RawMessage, tx *explorerPB.TxData, hashMap map[string]string) []types.CSVRecord {
	msgType := getMessageType(rawMsg)

	switch msgType {
	case "/cosmos.authz.v1beta1.MsgExec":
		return handleAuthzExec(rawMsg, tx, hashMap)

	case "/injective.exchange.v1beta1.MsgBatchUpdateOrders":
		return handleBatchUpdateOrders(rawMsg, tx, hashMap)

	case "/injective.exchange.v1beta1.MsgCreateDerivativeLimitOrder",
		"/injective.exchange.v1beta1.MsgCreateDerivativeMarketOrder":
		return handleCreateDerivativeOrder(rawMsg, msgType, tx, hashMap)

	case "/injective.exchange.v1beta1.MsgCancelDerivativeOrder":
		return handleCancelDerivativeOrder(rawMsg, tx)

	default:
		return nil
	}
}

func handleAuthzExec(rawMsg json.RawMessage, tx *explorerPB.TxData, hashMap map[string]string) []types.CSVRecord {
	var wrapper types.AuthzMsgExecWrapper
	if err := json.Unmarshal(rawMsg, &wrapper); err != nil {
		log.Printf("Failed to unmarshal MsgExec: %v", err)
		return nil
	}

	var records []types.CSVRecord
	for _, sub := range wrapper.Value.Msgs {
		records = append(records, handleMessage(sub, tx, hashMap)...)
	}
	return records
}

func handleBatchUpdateOrders(rawMsg json.RawMessage, tx *explorerPB.TxData, hashMap map[string]string) []types.CSVRecord {
	var batchMsg types.MsgBatchUpdateOrders
	if err := json.Unmarshal(rawMsg, &batchMsg); err != nil {
		log.Printf("Failed to unmarshal MsgBatchUpdateOrders: %v", err)
		return nil
	}

	var records []types.CSVRecord

	// 1) Orders to create
	for _, o := range batchMsg.DerivativeOrdersToCreate {
		key := o.OrderInfo.SubaccountID + "|" + o.OrderInfo.Cid
		rec := types.CSVRecord{
			TxHash:       tx.Hash,
			Block:        tx.BlockNumber,
			Action:       "PLACE_ORDER",
			MarketID:     o.MarketID,
			Price:        o.OrderInfo.Price,
			Quantity:     o.OrderInfo.Quantity,
			OrderType:    o.OrderType,
			SubaccountID: o.OrderInfo.SubaccountID,
			Margin:       o.Margin,
			OrderHash:    hashMap[key],
		}
		records = append(records, rec)
	}

	// 2) Orders to cancel
	for _, c := range batchMsg.DerivativeOrdersToCancel {
		rec := types.CSVRecord{
			TxHash:       tx.Hash,
			Block:        tx.BlockNumber,
			Action:       "CANCEL_ORDER",
			MarketID:     c.MarketID,
			SubaccountID: c.SubaccountID,
		}
		if c.OrderHash != "" {
			rec.OrderHash = c.OrderHash
		} else {
			key := c.SubaccountID + "|" + c.Cid
			rec.OrderHash = hashMap[key]
		}
		records = append(records, rec)
	}

	return records
}

func handleCreateDerivativeOrder(rawMsg json.RawMessage, msgType string, tx *explorerPB.TxData, hashMap map[string]string) []types.CSVRecord {
	var msgOrder types.MsgCreateDerivativeOrder
	if err := json.Unmarshal(rawMsg, &msgOrder); err != nil {
		log.Printf("Failed to unmarshal MsgCreateDerivativeOrder: %v", err)
		return nil
	}

	key := msgOrder.OrderInfo.SubaccountId + "|" + msgOrder.OrderInfo.Cid
	rec := types.CSVRecord{
		TxHash:       tx.Hash,
		Block:        tx.BlockNumber,
		Action:       "PLACE_ORDER",
		MarketID:     msgOrder.MarketId,
		Price:        msgOrder.OrderInfo.Price,
		Quantity:     msgOrder.OrderInfo.Quantity,
		OrderType:    msgOrder.OrderType,
		SubaccountID: msgOrder.OrderInfo.SubaccountId,
		Margin:       msgOrder.Margin,
		OrderHash:    hashMap[key],
	}
	return []types.CSVRecord{rec}
}

func handleCancelDerivativeOrder(rawMsg json.RawMessage, tx *explorerPB.TxData) []types.CSVRecord {
	var msgCancel struct {
		Type         string `json:"@type"`
		MarketId     string `json:"market_id"`
		SubaccountId string `json:"subaccount_id"`
		OrderHash    string `json:"order_hash"`
		OrderMask    uint32 `json:"order_mask"`
	}
	if err := json.Unmarshal(rawMsg, &msgCancel); err != nil {
		log.Printf("Failed to unmarshal MsgCancelDerivativeOrder: %v", err)
		return nil
	}

	rec := types.CSVRecord{
		TxHash:       tx.Hash,
		Block:        tx.BlockNumber,
		Action:       "CANCEL_ORDER",
		MarketID:     msgCancel.MarketId,
		SubaccountID: msgCancel.SubaccountId,
		OrderHash:    msgCancel.OrderHash,
	}
	return []types.CSVRecord{rec}
}

func getMessageType(rawMsg json.RawMessage) string {
	var meta struct {
		Type  string `json:"type"`
		AType string `json:"@type"`
	}
	_ = json.Unmarshal(rawMsg, &meta)
	if meta.Type != "" {
		return meta.Type
	}
	return meta.AType
}
