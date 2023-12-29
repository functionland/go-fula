package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (bl *FxBlockchain) Seeded(ctx context.Context, to peer.ID, r SeededRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionSeeded, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) AccountCreate(ctx context.Context, to peer.ID) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionAccountCreate, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)

	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) AccountFund(ctx context.Context, to peer.ID, r AccountFundRequest) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionAccountFund, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)

	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) AccountExists(ctx context.Context, to peer.ID, r AccountExistsRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionAccountExists, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) AccountBalance(ctx context.Context, to peer.ID, r AccountBalanceRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionAccountBalance, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) HandleSeeded(ctx context.Context, req *SeededRequest) (string, error) {
	// Call manifestBatchStore method
	responseBody, statusCode, err := bl.callBlockchain(ctx, "POST", actionSeeded, req)
	if err != nil {
		// If there's an error, return it directly without processing the response body
		return "", fmt.Errorf("blockchain call error: %w, status code: %d", err, statusCode)
	}

	// Check if the status code is OK; if not, handle it as an error
	if statusCode != http.StatusOK {
		var errMsg map[string]interface{}
		if jsonErr := json.Unmarshal(responseBody, &errMsg); jsonErr == nil {
			// If the responseBody is JSON, use it in the error message
			return "", fmt.Errorf("unexpected response status: %d, message: %s, description: %s",
				statusCode, errMsg["message"], errMsg["description"])
		} else {
			// If the responseBody is not JSON, return it as a plain text error message
			return "", fmt.Errorf("unexpected response status: %d, body: %s", statusCode, string(responseBody))
		}
	}

	// If the status code is 200, interpret the response
	var seededResponse SeededResponse
	if err := json.Unmarshal(responseBody, &seededResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal seededResponse response: %w", err)
	}
	return seededResponse.Account, nil
}

func isValidAccountFormat(account string) bool {
	// This is an example pattern: exactly 48 alphanumeric characters.
	// Modify the regex according to your actual account format requirements.
	re := regexp.MustCompile(`^[a-zA-Z0-9]{48}$`)
	return re.MatchString(account)
}

func (bl *FxBlockchain) HandleGetAccount(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	if bl.isAccountCached && bl.cachedAccount != "" {
		json.NewEncoder(w).Encode(map[string]string{"account": bl.cachedAccount})
		return
	}

	// Call HandleSeeded to get the account
	seededReq := SeededRequest{}
	account, err := bl.HandleSeeded(ctx, &seededReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting account from seed: %v", err), http.StatusInternalServerError)
		return
	}

	if !isValidAccountFormat(account) {
		http.Error(w, fmt.Sprintf("invalid account format: %s", account), http.StatusBadRequest)
		return
	}

	// Cache the account for future use
	bl.cachedAccount = account
	bl.isAccountCached = true

	// Send the account as JSON response
	json.NewEncoder(w).Encode(map[string]string{"account": account})
}

func (bl *FxBlockchain) AssetsBalance(ctx context.Context, to peer.ID, r AssetsBalanceRequest) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionAssetsBalance, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	}

	return b, nil
}

func (bl *FxBlockchain) TransferToFula(ctx context.Context, to peer.ID, r TransferToFulaRequest) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}
	action := ""
	switch r.Chain {
	case "mumbai":
		action = actionTransferToMumbai
	case "goerli":
		action = actionTransferToGoerli
	default:
		action = ""
	}

	if action != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+action, &buf)
		if err != nil {
			return nil, err
		}
		resp, err := bl.c.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusAccepted {
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}

		return b, nil
	}
	return []byte{}, errors.New("selected chain is not supported yet")
}
