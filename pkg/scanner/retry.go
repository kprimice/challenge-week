package scanner

import (
	"context"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	explorerclient "github.com/InjectiveLabs/sdk-go/client/explorer"
	explorerPB "github.com/InjectiveLabs/sdk-go/exchange/explorer_rpc/pb"
)

// GetTxsWithRetry tries up to N times to call GetTxs
func GetTxsWithRetry(
	ctx context.Context,
	client explorerclient.ExplorerClient,
	req *explorerPB.GetTxsRequest,
	attempts int,
) (*explorerPB.GetTxsResponse, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		res, err := client.GetTxs(ctx, req)
		if err == nil {
			return res, nil // success
		}

		log.Printf("[Retry %d/%d] GetTxs error: %v", i+1, attempts, err)
		lastErr = err

		// If error is transient, wait & retry
		if isTransientError(err) {
			backoff := time.Duration(1+i) * time.Second
			time.Sleep(backoff)
			continue
		}
		// Otherwise return immediately
		return nil, err
	}
	return nil, lastErr
}

// isTransientError checks for typical "unavailable" or "connection refused"
func isTransientError(err error) bool {
	st, ok := status.FromError(err)
	if ok && st.Code() == codes.Unavailable {
		return true
	}
	if strings.Contains(err.Error(), "connection refused") {
		return true
	}
	return false
}
