package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/functionland/go-fula/blockchain/abi"
	"github.com/functionland/go-fula/common"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

type PingResponse struct {
	Success bool   `json:"Success"`
	Time    int64  `json:"Time"`
	Text    string `json:"Text"`
}

func (bl *FxBlockchain) PoolCreate(ctx context.Context, to peer.ID, r PoolCreateRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionPoolCreate, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) HandlePoolJoin(method string, action string, from peer.ID, w http.ResponseWriter, r *http.Request) {
	// This handles the join request sent from client for a blox which is not part of the pool yet
	log := log.With("action", action, "from", from)
	var req PoolJoinRequest
	var res PoolJoinResponse

	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("cannot parse request body: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*time.Duration(bl.timeout))
	defer cancel()

	poolID := req.PoolID
	poolIDStr := strconv.Itoa(poolID)
	chainName := req.ChainName

	// If no chain name provided, determine it automatically
	if chainName == "" {
		log.Debugw("No chain name provided, attempting auto-discovery", "poolID", poolID)

		// Try to discover which chain this pool exists on
		if discoveredChain, err := bl.discoverPoolChain(ctx, uint32(poolID)); err == nil {
			chainName = discoveredChain
			log.Infow("Auto-discovered chain for pool", "poolID", poolID, "chain", chainName)
		} else {
			// Default to skale if discovery fails
			chainName = "skale"
			log.Warnw("Failed to discover chain, defaulting to skale", "poolID", poolID, "error", err)
		}
	}

	// Validate that the pool exists on the specified chain
	if err := bl.validatePoolOnChain(ctx, uint32(poolID), chainName); err != nil {
		errMsg := map[string]interface{}{
			"message":     "Pool validation failed",
			"description": fmt.Sprintf("Pool %d does not exist on chain %s: %s", poolID, chainName, err.Error()),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errMsg)
		return
	}

	// Update pool name configuration
	if errUpdatePool := bl.updatePoolName(poolIDStr); errUpdatePool != nil {
		errMsg := map[string]interface{}{
			"message":     "Pool Join is submitted but Error in updating Pool Config",
			"description": fmt.Sprintf("Error in updatePoolName: %s", errUpdatePool.Error()),
		}
		w.WriteHeader(http.StatusExpectationFailed)
		json.NewEncoder(w).Encode(errMsg)
		return
	}

	// Update chain name configuration
	if errUpdateChain := bl.updateChainName(chainName); errUpdateChain != nil {
		errMsg := map[string]interface{}{
			"message":     "Pool Join is submitted but Error in updating Chain Config",
			"description": fmt.Sprintf("Error in updateChainName: %s", errUpdateChain.Error()),
		}
		w.WriteHeader(http.StatusExpectationFailed)
		json.NewEncoder(w).Encode(errMsg)
		return
	}

	recordPoolJoinTime(time.Now())

	statusCode := http.StatusAccepted
	res = PoolJoinResponse{
		Account:   "",
		PoolID:    req.PoolID,
		ChainName: chainName,
	}

	log.Infow("Pool join request processed successfully", "poolID", poolID, "chain", chainName, "from", from)

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Error("failed to write response: %v", err)
	}
}

func (bl *FxBlockchain) PoolJoin(ctx context.Context, to peer.ID, r PoolJoinRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionPoolJoin, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) StartPingServer(ctx context.Context) error {
	return bl.StartPingProxy(ctx)
}

func (bl *FxBlockchain) StopPingServer(ctx context.Context) error {
	return bl.ShutdownPingProxy(ctx)
}

func (bl *FxBlockchain) PoolCancelJoin(ctx context.Context, to peer.ID, r PoolCancelJoinRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionPoolCancelJoin, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) HandlePoolCancelJoin(method string, action string, from peer.ID, w http.ResponseWriter, r *http.Request) {
	// This handles the join request sent from client for a blox which is not part of the pool yet
	log := log.With("action", action, "from", from)
	var req PoolCancelJoinRequest
	var res PoolCancelJoinResponse

	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("cannot parse request body: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*time.Duration(bl.timeout))
	defer cancel()
	response, statusCode, err := bl.callBlockchain(ctx, method, action, &req)
	if err != nil {
		poolID := req.PoolID
		poolIDStr := strconv.Itoa(poolID)
		requestSubmitted, err := bl.checkIfUserHasOpenPoolRequests(ctx, poolIDStr)
		if err == nil && !requestSubmitted {
			errUpdatePool := bl.updatePoolName("0")
			errUpdateChain := bl.updateChainName("")
			errPingServer := bl.StopPingServer(ctx)

			var errorMsgs []string
			if errUpdatePool != nil {
				errorMsgs = append(errorMsgs, fmt.Sprintf("Error in updatePoolName: %s", errUpdatePool.Error()))
			}
			if errUpdateChain != nil {
				errorMsgs = append(errorMsgs, fmt.Sprintf("Error in updateChainName: %s", errUpdateChain.Error()))
			}
			if errPingServer != nil {
				errorMsgs = append(errorMsgs, fmt.Sprintf("Error in Ping server: %s", errPingServer.Error()))
			}

			if len(errorMsgs) > 0 {
				errMsg := map[string]interface{}{
					"message":     "Pool Cancel Join is submitted but Error in cleanup",
					"description": strings.Join(errorMsgs, ", "),
				}
				w.WriteHeader(http.StatusExpectationFailed)
				json.NewEncoder(w).Encode(errMsg)
				return
			} else {
				statusCode = http.StatusAccepted
				err = nil
				response = []byte(fmt.Sprintf("{\"account\":\"\",\"pool_id\":%d}", req.PoolID))
			}
		}
	}
	if statusCode == http.StatusOK {
		statusCode = http.StatusAccepted
	}
	log.Debugw("callblockchain response in PoolCancelJoin", "statusCode", statusCode, "response", response, "err", err)
	// If status code is not 200, attempt to format the response as JSON
	if statusCode != http.StatusAccepted || err != nil {
		w.WriteHeader(statusCode)
		// Try to parse the error and format it as JSON
		var errMsg map[string]interface{}
		if jsonErr := json.Unmarshal(response, &errMsg); jsonErr != nil {
			// If the response isn't JSON or can't be parsed, use a generic message
			errMsg = map[string]interface{}{
				"message":     "An error occurred",
				"description": err.Error(),
			}
		}
		json.NewEncoder(w).Encode(errMsg)
		return
	}

	bl.cleanLeaveJoinPool(ctx, req.PoolID)
	w.WriteHeader(statusCode)
	err1 := json.Unmarshal(response, &res)
	if err1 != nil {
		log.Error("failed to format response: %v", err1)
	}

	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Error("failed to write response: %v", err)
	}
}

func (bl *FxBlockchain) cleanLeaveJoinPool(ctx context.Context, PoolID int) {
	// Reset pool configuration
	if bl.updatePoolName != nil {
		if err := bl.updatePoolName("0"); err != nil {
			log.Errorw("Failed to reset pool name", "error", err)
		}
	}

	// Reset chain configuration
	if bl.updateChainName != nil {
		if err := bl.updateChainName(""); err != nil {
			log.Errorw("Failed to reset chain name", "error", err)
		}
	}
	bl.topicName = "0"
	bl.chainName = ""
	clearPoolJoinTime()

	bl.StopPingServer(ctx)
	// Announcements no longer used - pool joins handled via blockchain API
	// Send a stop signal if the channel is not nil
	if bl.stopFetchUsersAfterJoinChan != nil {
		close(bl.stopFetchUsersAfterJoinChan)
		// Reset the channel to nil to avoid closing a closed channel
		bl.stopFetchUsersAfterJoinChan = nil
	}
	bl.clearPoolPeersFromPeerAddr(ctx, PoolID)
}

func (bl *FxBlockchain) PoolRequests(ctx context.Context, to peer.ID, r PoolRequestsRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionPoolRequests, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) PoolList(ctx context.Context, to peer.ID, r PoolListRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionPoolList, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) ReplicateInPool(ctx context.Context, to peer.ID, r ReplicateRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionReplicateInPool, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) PoolUserList(ctx context.Context, to peer.ID, r PoolUserListRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionPoolUserList, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) PoolVote(ctx context.Context, to peer.ID, r PoolVoteRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionPoolVote, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) PoolLeave(ctx context.Context, to peer.ID, r PoolLeaveRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionPoolLeave, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) HandlePoolJoinRequest(ctx context.Context, from peer.ID, account string, topicString string, withMemberListUpdate bool) error {
	// Note: withMemberListUpdate parameter is deprecated - no longer needed with EVM chain integration
	// Voting logic has been removed as it's no longer required

	averageDuration := float64(2000)
	successCount := 0

	// Check if peer exists in our local members map
	status, exists := bl.GetMemberStatus(from)
	if !exists {
		log.Debugw("PeerID not found in local members map, this is expected with EVM integration", "peerID", from)
		// With EVM integration, we don't maintain a local members list
		// This method is now primarily for legacy compatibility
		return nil
	}
	if status == common.Pending {
		// Ping
		/*
			// Use IPFS Ping
			log.Debugw("****** Pinging pending node", "from", bl.selfPeerID, "to", from)
			averageDuration, successCount, err := bl.p.Ping(ctx, from)
			if err != nil {
				log.Errorw("An error occurred during ping", "error", err)
				return err
			}
			if bl.maxPingTime == 0 {
				//TODO: This should not happen but is happening!
				bl.maxPingTime = 200
			}
			if bl.minPingSuccessCount == 0 {
				//TODO: This should not happen but is happening!
				bl.minPingSuccessCount = 3
			}
		*/
		// Set up the request with context for timeout
		ctxPing, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if bl.rpc == nil {
			return fmt.Errorf("IPFS rpc is not defined")
		}
		// Send the ping request
		res, err := bl.rpc.Request("ping", from.String()).Send(ctxPing)
		if err != nil {
			return fmt.Errorf("ping was unsuccessful: %s", err)
		}
		if res.Error != nil {
			return fmt.Errorf("ping was unsuccessful: %s", res.Error)
		}

		// Process multiple responses using a decoder
		decoder := json.NewDecoder(res.Output)

		var totalTime int64
		var count int

		for decoder.More() {
			var pingResp PingResponse
			err := decoder.Decode(&pingResp)
			if err != nil {
				log.Errorf("error decoding JSON response: %s", err)
				continue
			}

			if pingResp.Text == "" && pingResp.Time > 0 { // Check for empty Text field and Time
				totalTime += pingResp.Time
				count++
			}
		}

		if count > 0 {
			averageDuration = float64(totalTime) / float64(count) / 1e6 // Convert nanoseconds to milliseconds
			successCount = count
		} else {
			fmt.Println("No valid ping responses received")
		}

		vote := int(averageDuration) <= bl.maxPingTime && successCount >= bl.minPingSuccessCount

		log.Debugw("Ping result", "averageDuration", averageDuration, "successCount", successCount, "vote", vote, "bl.maxPingTime", bl.maxPingTime, "bl.minPingSuccessCount", bl.minPingSuccessCount)

		// Convert topic from string to int
		poolID, err := strconv.Atoi(topicString)
		if err != nil {
			return fmt.Errorf("invalid topic, not an integer: %s", err)
		}
		voteRequest := PoolVoteRequest{
			PoolID:    poolID,
			Account:   account,
			PeerID:    from.String(),
			VoteValue: vote,
		}

		// Call PoolVote method
		responseBody, statusCode, err := bl.callBlockchain(ctx, "POST", actionPoolVote, &voteRequest)
		if err != nil {
			return fmt.Errorf("blockchain call error: %w, status code: %d", err, statusCode)
		}

		// Check if the status code is OK; if not, handle it as an error
		if statusCode != http.StatusOK && statusCode != http.StatusAccepted {
			var errMsg map[string]interface{}
			if jsonErr := json.Unmarshal(responseBody, &errMsg); jsonErr == nil {
				return fmt.Errorf("unexpected response status: %d, message: %s, description: %s",
					statusCode, errMsg["message"], errMsg["description"])
			} else {
				return fmt.Errorf("unexpected response status: %d, body: %s", statusCode, string(responseBody))
			}
		}

		// Interpret the response
		var voteResponse PoolVoteResponse
		if err := json.Unmarshal(responseBody, &voteResponse); err != nil {
			return fmt.Errorf("failed to unmarshal vote response: %w", err)
		}

		// Handle the response as needed
		log.Infow("Vote cast successfully", "statusCode", statusCode, "voteResponse", voteResponse, "on", from, "by", bl.selfPeerID)
		// Update member status to unknown
		bl.membersLock.Lock()
		bl.members[from] = common.Unknown
		bl.membersLock.Unlock()
	} else {
		return fmt.Errorf("peerID does not exist in the list of pool requests: %s with status %d", from, status)
	}
	return nil
}

func (bl *FxBlockchain) HandlePoolList(ctx context.Context) (PoolListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(bl.timeout))
	defer cancel()

	req := PoolListRequest{}
	var res PoolListResponse
	response, _, err := bl.callBlockchain(ctx, "POST", actionPoolList, req)
	if err != nil {
		return PoolListResponse{}, err
	}
	err = json.Unmarshal(response, &res)
	if err != nil {
		log.Error("failed to format response: %v", err)
		return PoolListResponse{}, err
	}
	return res, nil
}

// HandleEVMPoolList retrieves pool list from EVM chains (Base/Skale)
func (bl *FxBlockchain) HandleEVMPoolList(ctx context.Context, chainName string) (EVMPoolListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(bl.timeout))
	defer cancel()

	chainConfigs := GetChainConfigs()
	chainConfig, exists := chainConfigs[chainName]
	if !exists {
		return EVMPoolListResponse{}, fmt.Errorf("unsupported chain: %s", chainName)
	}

	var pools []EVMPool
	index := uint32(0) // Start from index 0 for poolIds enumeration

	log.Debugw("Starting pool discovery using poolIds(index)", "chain", chainName, "startIndex", index)

	for {
		// Call the poolIds(uint256) method on the contract using index-based enumeration
		callData := abi.EncodePoolIdsCall(index)
		params := []interface{}{
			map[string]interface{}{
				"to":   chainConfig.Contract,
				"data": callData,
			},
			"latest",
		}

		response, statusCode, err := bl.callEVMChainWithRetry(ctx, chainName, "eth_call", params, 3)
		if err != nil {
			log.Debugw("Error calling poolIds - no more pools", "chain", chainName, "index", index, "error", err)
			break // Stop on error - no more pools
		}

		if statusCode != 200 {
			return EVMPoolListResponse{}, fmt.Errorf("unexpected status code %d from chain %s", statusCode, chainName)
		}

		// Parse JSON-RPC response
		var rpcResponse struct {
			Result string `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal(response, &rpcResponse); err != nil {
			return EVMPoolListResponse{}, fmt.Errorf("failed to parse RPC response: %w", err)
		}

		if rpcResponse.Error != nil {
			log.Debugw("RPC error calling poolIds - no more pools", "chain", chainName, "index", index, "error", rpcResponse.Error.Message)
			break // Stop on RPC error - no more pools
		}

		// Check if response indicates no pool at this index (empty or revert)
		if rpcResponse.Result == "0x" || len(rpcResponse.Result) <= 2 {
			log.Debugw("No pool at index - stopping discovery", "chain", chainName, "index", index)
			break
		}

		// Decode the pool ID from the response
		poolID, err := abi.DecodePoolIdResponse(rpcResponse.Result)
		if err != nil {
			log.Debugw("Error decoding poolIds response", "chain", chainName, "index", index, "error", err)
			break
		}

		// Skip pool 0 if it somehow appears
		if poolID == 0 {
			log.Debugw("Skipping pool 0 as it doesn't exist", "chain", chainName, "index", index)
			index++
			continue
		}

		log.Debugw("Found pool ID at index", "chain", chainName, "poolID", poolID, "index", index)

		// Now get the full pool data using pools(uint32) method
		poolCallData := abi.EncodePoolsCall(poolID)
		poolParams := []interface{}{
			map[string]interface{}{
				"to":   chainConfig.Contract,
				"data": poolCallData,
			},
			"latest",
		}

		poolResponse, poolStatusCode, err := bl.callEVMChainWithRetry(ctx, chainName, "eth_call", poolParams, 3)
		if err != nil {
			log.Warnw("Failed to get pool data", "chain", chainName, "poolID", poolID, "error", err)
			index++
			continue // Skip this pool but continue with next index
		}

		if poolStatusCode != 200 {
			log.Warnw("Unexpected status code getting pool data", "chain", chainName, "poolID", poolID, "statusCode", poolStatusCode)
			index++
			continue
		}

		// Parse pool data response
		var poolRpcResponse struct {
			Result string `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal(poolResponse, &poolRpcResponse); err != nil {
			log.Warnw("Failed to parse pool data response", "chain", chainName, "poolID", poolID, "error", err)
			index++
			continue
		}

		if poolRpcResponse.Error != nil {
			log.Warnw("RPC error getting pool data", "chain", chainName, "poolID", poolID, "error", poolRpcResponse.Error.Message)
			index++
			continue
		}

		// Decode the pool data using proper ABI decoding
		poolData, err := abi.DecodePoolsResult(poolRpcResponse.Result)
		if err != nil {
			log.Warnw("Failed to decode pool data", "chain", chainName, "poolID", poolID, "error", err)
			index++
			continue
		}

		// Convert to EVMPool format
		// IMPORTANT: Use the discovered poolID from poolIds() call, not poolData.ID which may be incorrectly parsed
		pool := EVMPool{
			Creator:                    poolData.Creator,
			ID:                         poolID, // Use the poolID we discovered from poolIds(), not poolData.ID
			MaxChallengeResponsePeriod: poolData.MaxChallengeResponsePeriod,
			MemberCount:                poolData.MemberCount,
			MaxMembers:                 poolData.MaxMembers,
			RequiredTokens:             poolData.RequiredTokens.String(),
			MinPingTime:                poolData.MinPingTime.String(),
			Name:                       poolData.Name,
			Region:                     poolData.Region,
			ChainName:                  chainName,
		}

		pools = append(pools, pool)
		index++

		// Safety check to prevent infinite loops
		if index > 20 {
			log.Warnw("Reached maximum index limit", "chain", chainName, "maxIndex", index)
			break
		}
	}

	return EVMPoolListResponse{Pools: pools}, nil
}

// HandleIsMemberOfPool checks if a peer is a member of a specific pool on a specific chain
func (bl *FxBlockchain) HandleIsMemberOfPool(ctx context.Context, req IsMemberOfPoolRequest) (IsMemberOfPoolResponse, error) {
	// Pool 0 doesn't exist, return false immediately
	if req.PoolID == 0 {
		log.Debugw("Pool 0 doesn't exist, returning false", "chain", req.ChainName, "peerID", req.PeerID)
		return IsMemberOfPoolResponse{
			IsMember:      false,
			MemberAddress: "0x0000000000000000000000000000000000000000",
			ChainName:     req.ChainName,
			PoolID:        req.PoolID,
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(bl.timeout))
	defer cancel()

	chainConfigs := GetChainConfigs()
	chainConfig, exists := chainConfigs[req.ChainName]
	if !exists {
		return IsMemberOfPoolResponse{}, fmt.Errorf("unsupported chain: %s", req.ChainName)
	}

	// Convert PeerID to bytes32
	peerIDBytes32, err := peerIdToBytes32(req.PeerID)
	if err != nil {
		return IsMemberOfPoolResponse{}, fmt.Errorf("failed to convert PeerID to bytes32: %w", err)
	}

	// Call isPeerIdMemberOfPool(uint32,bytes32) method using proper ABI encoding
	callData := abi.EncodeIsPeerIdMemberOfPoolCall(req.PoolID, peerIDBytes32)

	params := []interface{}{
		map[string]interface{}{
			"to":   chainConfig.Contract,
			"data": callData,
		},
		"latest",
	}

	response, statusCode, err := bl.callEVMChainWithRetry(ctx, req.ChainName, "eth_call", params, 3)
	if err != nil {
		log.Errorw("Failed to call EVM chain for membership check after retries", "chain", req.ChainName, "poolID", req.PoolID, "peerID", req.PeerID, "error", err)
		return IsMemberOfPoolResponse{
			IsMember:      false,
			MemberAddress: "0x0000000000000000000000000000000000000000",
			ChainName:     req.ChainName,
			PoolID:        req.PoolID,
		}, nil // Return false instead of error for graceful degradation
	}

	if statusCode != 200 {
		return IsMemberOfPoolResponse{}, fmt.Errorf("unexpected status code %d from chain %s", statusCode, req.ChainName)
	}

	// Parse JSON-RPC response
	var rpcResponse struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(response, &rpcResponse); err != nil {
		return IsMemberOfPoolResponse{}, fmt.Errorf("failed to parse RPC response: %w", err)
	}

	if rpcResponse.Error != nil {
		return IsMemberOfPoolResponse{}, fmt.Errorf("RPC error: %s", rpcResponse.Error.Message)
	}

	// Check for contract-specific errors
	if contractErr := abi.ParseContractError(rpcResponse.Result); contractErr != nil {
		log.Debugw("Contract error in membership check", "chain", req.ChainName, "poolID", req.PoolID, "error", contractErr)
		return IsMemberOfPoolResponse{
			IsMember:      false,
			MemberAddress: "0x0000000000000000000000000000000000000000",
			ChainName:     req.ChainName,
			PoolID:        req.PoolID,
		}, nil // Return false for contract errors (graceful degradation)
	}

	// Decode the result using proper ABI decoding
	membershipResult, err := abi.DecodeIsMemberOfPoolResult(rpcResponse.Result)
	if err != nil {
		log.Errorw("Failed to decode membership result", "chain", req.ChainName, "poolID", req.PoolID, "error", err)
		return IsMemberOfPoolResponse{
			IsMember:      false,
			MemberAddress: "0x0000000000000000000000000000000000000000",
			ChainName:     req.ChainName,
			PoolID:        req.PoolID,
		}, nil
	}

	return IsMemberOfPoolResponse{
		IsMember:      membershipResult.IsMember,
		MemberAddress: membershipResult.MemberAddress,
		ChainName:     req.ChainName,
		PoolID:        req.PoolID,
	}, nil
}

// GetPoolCreatorPeerID retrieves the creator's peer ID for a specific pool on a specific chain
func (bl *FxBlockchain) GetPoolCreatorPeerID(ctx context.Context, poolID uint32, chainName string) (string, error) {
	if poolID == 0 {
		return "", fmt.Errorf("invalid pool ID: %d", poolID)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(bl.timeout))
	defer cancel()

	chainConfigs := GetChainConfigs()
	chainConfig, exists := chainConfigs[chainName]
	if !exists {
		return "", fmt.Errorf("unsupported chain: %s", chainName)
	}

	log.Debugw("Getting pool creator peer ID", "poolID", poolID, "chain", chainName)

	// Step 1: Get pool information to find the creator
	poolCallData := abi.EncodePoolsCall(poolID)
	poolParams := []interface{}{
		map[string]interface{}{
			"to":   chainConfig.Contract,
			"data": poolCallData,
		},
		"latest",
	}

	poolResponse, statusCode, err := bl.callEVMChainWithRetry(ctx, chainName, "eth_call", poolParams, 3)
	if err != nil {
		log.Errorw("Failed to get pool information", "chain", chainName, "poolID", poolID, "error", err)
		return "", fmt.Errorf("failed to get pool information: %w", err)
	}

	if statusCode != 200 {
		return "", fmt.Errorf("unexpected status code %d from chain %s", statusCode, chainName)
	}

	// Parse pool response
	var poolRPCResponse struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(poolResponse, &poolRPCResponse); err != nil {
		return "", fmt.Errorf("failed to parse pool RPC response: %w", err)
	}

	if poolRPCResponse.Error != nil {
		return "", fmt.Errorf("pool RPC error: %s", poolRPCResponse.Error.Message)
	}

	// Check for contract-specific errors
	if contractErr := abi.ParseContractError(poolRPCResponse.Result); contractErr != nil {
		return "", fmt.Errorf("pool contract error: %w", contractErr)
	}

	// Decode pool data
	poolData, err := abi.DecodePoolsResult(poolRPCResponse.Result)
	if err != nil {
		return "", fmt.Errorf("failed to decode pool data: %w", err)
	}

	// Step 2: Get creator's peer IDs
	peerIdsCallData := abi.EncodeGetMemberPeerIdsCall(poolID, poolData.Creator)
	peerIdsParams := []interface{}{
		map[string]interface{}{
			"to":   chainConfig.Contract,
			"data": peerIdsCallData,
		},
		"latest",
	}

	peerIdsResponse, statusCode, err := bl.callEVMChainWithRetry(ctx, chainName, "eth_call", peerIdsParams, 3)
	if err != nil {
		log.Errorw("Failed to get creator peer IDs", "chain", chainName, "poolID", poolID, "creator", poolData.Creator, "error", err)
		return "", fmt.Errorf("failed to get creator peer IDs: %w", err)
	}

	if statusCode != 200 {
		return "", fmt.Errorf("unexpected status code %d from chain %s for peer IDs", statusCode, chainName)
	}

	// Parse peer IDs response
	var peerIdsRPCResponse struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(peerIdsResponse, &peerIdsRPCResponse); err != nil {
		return "", fmt.Errorf("failed to parse peer IDs RPC response: %w", err)
	}

	if peerIdsRPCResponse.Error != nil {
		return "", fmt.Errorf("peer IDs RPC error: %s", peerIdsRPCResponse.Error.Message)
	}

	// Check for contract-specific errors
	if contractErr := abi.ParseContractError(peerIdsRPCResponse.Result); contractErr != nil {
		return "", fmt.Errorf("peer IDs contract error: %w", contractErr)
	}

	// Decode peer IDs data
	peerIdsData, err := abi.DecodeGetMemberPeerIdsResult(peerIdsRPCResponse.Result)
	if err != nil {
		return "", fmt.Errorf("failed to decode peer IDs data: %w", err)
	}

	// Step 3: Convert the first peer ID from bytes32 to full peer ID
	if len(peerIdsData.PeerIds) == 0 {
		return "", fmt.Errorf("no peer IDs found for pool creator %s in pool %d", poolData.Creator, poolID)
	}

	// Use the first peer ID (as specified in requirements)
	firstPeerIDBytes32 := peerIdsData.PeerIds[0]

	// Validate the bytes32 peer ID
	if firstPeerIDBytes32 == "" || firstPeerIDBytes32 == "0x0000000000000000000000000000000000000000000000000000000000000000" {
		return "", fmt.Errorf("invalid or empty peer ID for pool creator %s in pool %d", poolData.Creator, poolID)
	}

	// Convert bytes32 to full peer ID
	fullPeerID, err := Bytes32ToPeerID(firstPeerIDBytes32)
	if err != nil {
		return "", fmt.Errorf("failed to convert bytes32 %s to peer ID: %w", firstPeerIDBytes32, err)
	}

	// Validate the converted peer ID
	if fullPeerID.String() == "" {
		return "", fmt.Errorf("converted peer ID is empty for bytes32 %s", firstPeerIDBytes32)
	}

	log.Infow("Successfully retrieved pool creator peer ID",
		"chain", chainName,
		"poolID", poolID,
		"creator", poolData.Creator,
		"bytes32PeerID", firstPeerIDBytes32,
		"fullPeerID", fullPeerID.String())

	return fullPeerID.String(), nil
}

// discoverPoolChain attempts to discover which chain a pool exists on
func (bl *FxBlockchain) discoverPoolChain(ctx context.Context, poolID uint32) (string, error) {
	chainList := []string{"skale", "base"}

	for _, chainName := range chainList {
		if err := bl.validatePoolOnChain(ctx, poolID, chainName); err == nil {
			return chainName, nil
		}
	}

	return "", fmt.Errorf("pool %d not found on any supported chain", poolID)
}

// validatePoolOnChain checks if a pool exists on a specific chain
func (bl *FxBlockchain) validatePoolOnChain(ctx context.Context, poolID uint32, chainName string) error {
	chainConfigs := GetChainConfigs()
	chainConfig, exists := chainConfigs[chainName]
	if !exists {
		return fmt.Errorf("unsupported chain: %s", chainName)
	}

	// Call the pools(uint32) method to check if pool exists
	callData := abi.EncodePoolsCall(poolID)
	params := []interface{}{
		map[string]interface{}{
			"to":   chainConfig.Contract,
			"data": callData,
		},
		"latest",
	}

	response, statusCode, err := bl.callEVMChainWithRetry(ctx, chainName, "eth_call", params, 2)
	if err != nil {
		return fmt.Errorf("failed to call chain %s: %w", chainName, err)
	}

	if statusCode != 200 {
		return fmt.Errorf("unexpected status code %d from chain %s", statusCode, chainName)
	}

	// Parse JSON-RPC response
	var rpcResponse struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(response, &rpcResponse); err != nil {
		return fmt.Errorf("failed to parse RPC response: %w", err)
	}

	if rpcResponse.Error != nil {
		return fmt.Errorf("RPC error: %s", rpcResponse.Error.Message)
	}

	// Check for contract-specific errors
	if contractErr := abi.ParseContractError(rpcResponse.Result); contractErr != nil {
		return fmt.Errorf("contract error: %w", contractErr)
	}

	// Try to decode the pool data to validate it exists
	_, err = abi.DecodePoolsResult(rpcResponse.Result)
	if err != nil {
		return fmt.Errorf("pool %d does not exist on chain %s: %w", poolID, chainName, err)
	}

	return nil
}

// HandlePoolLeave handles pool leave requests with chain support.
//
// The on-chain removal (removeMemberPeerId) is sent by the app from the member's wallet — the device has no key to
// sign it — so this handler only drops the pool from the local config, and only once the chain confirms the device's
// cluster peer is neither a member of the pool nor waiting to join it:
//   - 202: removed on-chain (or never there) → pool and chain cleared from config, fula services restarted so
//     ipfs-cluster leaves the old pool;
//   - 202 without changes when no pool is configured;
//   - 409: the device is configured for a different pool or chain, or the chain still lists it (the app's tx not
//     mined yet);
//   - 503: the chain could not be read, or the cluster peer id is unknown — config untouched, safe to retry.
func (bl *FxBlockchain) HandlePoolLeave(method string, action string, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", action, "from", from)
	var req PoolLeaveRequest
	var res PoolLeaveResponse

	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("cannot parse request body: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*time.Duration(bl.timeout))
	defer cancel()

	writeError := func(status int, message, description string) {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": message, "description": description})
	}

	poolID := req.PoolID
	chainName := req.ChainName

	// Never clear a different pool than the one asked about.
	configuredPool := "0"
	if bl.getPoolName != nil {
		configuredPool = bl.getPoolName()
	}
	if configuredPool != "" && configuredPool != "0" && configuredPool != strconv.Itoa(poolID) {
		writeError(http.StatusConflict, "Pool mismatch",
			fmt.Sprintf("This Blox is configured for pool %s, not pool %d", configuredPool, poolID))
		return
	}
	// Nothing configured (e.g. a reconcile already cleared it): nothing to clean up or restart.
	if configuredPool == "" || configuredPool == "0" {
		log.Infow("Pool leave: no pool configured, nothing to do", "poolID", poolID, "chain", chainName)
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(PoolLeaveResponse{PoolID: req.PoolID, ChainName: chainName}); err != nil {
			log.Error("failed to write response: %v", err)
		}
		return
	}
	configuredChain := ""
	if bl.getChainName != nil {
		configuredChain = bl.getChainName()
	}

	// If no chain name provided, try to determine it from current configuration
	if chainName == "" {
		if configuredChain != "" {
			chainName = configuredChain
			log.Debugw("Using current chain configuration", "poolID", poolID, "chain", chainName)
		} else {
			// Try to discover which chain this pool exists on
			log.Debugw("No chain name provided, attempting auto-discovery", "poolID", poolID)
			if discoveredChain, err := bl.discoverPoolChain(ctx, uint32(poolID)); err == nil {
				chainName = discoveredChain
				log.Infow("Auto-discovered chain for pool", "poolID", poolID, "chain", chainName)
			} else {
				// Default to skale if discovery fails
				chainName = "skale"
				log.Warnw("Failed to discover chain, defaulting to skale", "poolID", poolID, "error", err)
			}
		}
	}
	// Pool ids repeat across chains (a Blox can even be in pool 1 on both): a leave on another chain than the one
	// this Blox is configured for must not clear its config.
	if configuredChain != "" && configuredChain != chainName {
		writeError(http.StatusConflict, "Chain mismatch",
			fmt.Sprintf("This Blox is configured for pool %s on %s, not on %s", configuredPool, configuredChain, chainName))
		return
	}

	// The app sends this right after its removeMemberPeerId tx is mined; a public RPC can lag a block or two.
	stillListed := false
	for attempt := 1; attempt <= poolLeaveCheckAttempts; attempt++ {
		member, pending, err := bl.ClusterPeerPoolStatus(ctx, uint32(poolID), chainName)
		if err != nil {
			log.Warnw("Pool leave: membership check failed", "poolID", poolID, "chain", chainName, "error", err)
			writeError(http.StatusServiceUnavailable, "Could not verify pool membership",
				fmt.Sprintf("Membership check on %s failed: %s", chainName, err.Error()))
			return
		}
		stillListed = member || pending
		if !stillListed || attempt == poolLeaveCheckAttempts {
			break
		}
		select {
		case <-ctx.Done():
			writeError(http.StatusServiceUnavailable, "Could not verify pool membership", ctx.Err().Error())
			return
		case <-time.After(poolLeaveCheckInterval):
		}
	}
	if stillListed {
		writeError(http.StatusConflict, "Still in the pool on-chain",
			fmt.Sprintf("This Blox is still a member of (or waiting to join) pool %d on %s; remove it on-chain first", poolID, chainName))
		return
	}

	// Clean up local configuration and state
	if err := bl.clearPoolConfig(ctx, poolID); err != nil {
		log.Errorw("Pool leave: could not clear the pool from the config", "poolID", poolID, "error", err)
		writeError(http.StatusInternalServerError, "Failed to update the Blox config", err.Error())
		return
	}

	statusCode := http.StatusAccepted
	res = PoolLeaveResponse{
		Account:   "",
		PoolID:    req.PoolID,
		ChainName: chainName,
	}

	log.Infow("Pool leave request processed successfully", "poolID", poolID, "chain", chainName, "from", from)

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Error("failed to write response: %v", err)
	}
	// Give the response time to reach the app before the services restart.
	bl.signalPoolConfigCleared(poolConfigClearedRestartDelay)
}

const (
	poolLeaveCheckAttempts = 3
	// PoolJoinGracePeriod is how long after a join the configured pool is kept even though the chain doesn't list
	// the Blox yet: a join writes the config immediately, but the join server's addMember (or approval votes for a
	// contract join) lands later.
	PoolJoinGracePeriod = 24 * time.Hour
	joinRequestPending  = 1 // JoinRequest.status of a request still waiting for votes
)

// Variables (not constants) so tests can shorten them.
var (
	poolLeaveCheckInterval        = 4 * time.Second
	poolConfigClearedRestartDelay = 3 * time.Second
)

// PoolJoinedAtFilePath records when HandlePoolJoin last wrote a pool into the config (see PoolJoinGracePeriod).
// A variable so tests can point it at a temp dir.
var PoolJoinedAtFilePath = "/internal/.tmp/pool_joined_at.tmp"

func recordPoolJoinTime(t time.Time) {
	if err := os.MkdirAll(filepath.Dir(PoolJoinedAtFilePath), 0755); err != nil {
		log.Warnw("Failed to create directory for pool join time", "err", err)
		return
	}
	if err := os.WriteFile(PoolJoinedAtFilePath, []byte(t.UTC().Format(time.RFC3339)), 0644); err != nil {
		log.Warnw("Failed to record pool join time", "err", err)
	}
}

func clearPoolJoinTime() {
	if err := os.Remove(PoolJoinedAtFilePath); err != nil && !os.IsNotExist(err) {
		log.Warnw("Failed to remove pool join time", "err", err)
	}
}

// PoolJoinedAt returns when the configured pool was joined from this device; false when unknown (e.g. joins made
// before this marker existed), which counts as an old join.
func PoolJoinedAt() (time.Time, bool) {
	b, err := os.ReadFile(PoolJoinedAtFilePath)
	if err != nil {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(b)))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// signalPoolConfigCleared runs the WithOnPoolConfigCleared hook (the blox wires it to a fula services restart, so
// ipfs-cluster re-reads the now empty pool) after delay, off the caller's goroutine.
func (bl *FxBlockchain) signalPoolConfigCleared(delay time.Duration) {
	if bl.onPoolConfigCleared == nil {
		return
	}
	hook := bl.onPoolConfigCleared
	go func() {
		time.Sleep(delay)
		hook()
	}()
}

// ClusterPeerPoolStatus reports whether this device's cluster peer is a member of poolID on chainName, and whether
// it has a pending join request there. Unlike HandleIsMemberOfPool (which treats RPC failures as "not a member"),
// any RPC or decoding failure is returned as an error, so callers never clear the config on a guess.
func (bl *FxBlockchain) ClusterPeerPoolStatus(ctx context.Context, poolID uint32, chainName string) (member bool, pending bool, err error) {
	if bl.clusterPeerID == "" {
		return false, false, fmt.Errorf("cluster peer ID is not set")
	}
	peerIDBytes32, err := peerIdToBytes32(bl.clusterPeerID.String())
	if err != nil {
		return false, false, fmt.Errorf("failed to convert cluster peer ID to bytes32: %w", err)
	}

	memberResult, err := bl.strictEthCall(ctx, chainName, abi.EncodeIsPeerIdMemberOfPoolCall(poolID, peerIDBytes32))
	if err != nil {
		return false, false, fmt.Errorf("isPeerIdMemberOfPool: %w", err)
	}
	membership, err := abi.DecodeIsMemberOfPoolResult(memberResult)
	if err != nil {
		return false, false, fmt.Errorf("isPeerIdMemberOfPool: %w", err)
	}
	if membership.IsMember {
		return true, false, nil
	}

	requestResult, err := bl.strictEthCall(ctx, chainName, abi.EncodeJoinRequestsCall(poolID, peerIDBytes32))
	if err != nil {
		return false, false, fmt.Errorf("joinRequests: %w", err)
	}
	status, err := abi.DecodeJoinRequestStatus(requestResult)
	if err != nil {
		return false, false, fmt.Errorf("joinRequests: %w", err)
	}
	return false, status == joinRequestPending, nil
}

// strictEthCall runs a read-only eth_call against the pool contract on chainName and returns the raw result; RPC,
// HTTP and JSON-RPC errors are all returned as errors.
func (bl *FxBlockchain) strictEthCall(ctx context.Context, chainName string, callData string) (string, error) {
	chainConfig, exists := GetChainConfigs()[chainName]
	if !exists {
		return "", fmt.Errorf("unsupported chain: %s", chainName)
	}
	params := []interface{}{
		map[string]interface{}{
			"to":   chainConfig.Contract,
			"data": callData,
		},
		"latest",
	}
	response, statusCode, err := bl.callEVMChainWithRetry(ctx, chainName, "eth_call", params, 3)
	if err != nil {
		return "", err
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d from chain %s", statusCode, chainName)
	}
	var rpcResponse struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response, &rpcResponse); err != nil {
		return "", fmt.Errorf("failed to parse RPC response: %w", err)
	}
	if rpcResponse.Error != nil {
		return "", fmt.Errorf("RPC error: %s", rpcResponse.Error.Message)
	}
	return rpcResponse.Result, nil
}

// ReconcilePoolConfig clears the configured pool when the chain definitively says this device is neither a member
// of it nor waiting to join it — e.g. it was removed from the app while offline, or the app's leave notice never
// arrived — unless the pool was joined from this device less than PoolJoinGracePeriod ago. Any read error keeps the
// config. Returns true when the config was cleared (and the services restart was signalled).
func (bl *FxBlockchain) ReconcilePoolConfig(ctx context.Context) (bool, error) {
	if bl.getPoolName == nil || bl.getChainName == nil {
		return false, nil
	}
	pool, chainName := bl.getPoolName(), bl.getChainName()
	if pool == "" || pool == "0" || chainName == "" {
		return false, nil
	}
	if joinedAt, ok := PoolJoinedAt(); ok && time.Since(joinedAt) < PoolJoinGracePeriod {
		log.Debugw("Pool reconcile: join is recent, keeping config", "pool", pool, "chain", chainName, "joinedAt", joinedAt)
		return false, nil
	}
	poolID, err := strconv.ParseUint(pool, 10, 32)
	if err != nil {
		return false, fmt.Errorf("invalid configured pool %q: %w", pool, err)
	}
	member, pending, err := bl.ClusterPeerPoolStatus(ctx, uint32(poolID), chainName)
	if err != nil {
		return false, err
	}
	if member || pending {
		return false, nil
	}
	log.Infow("Pool reconcile: this Blox is no longer in the configured pool on-chain; clearing it", "pool", pool, "chain", chainName)
	if err := bl.clearPoolConfig(ctx, int(poolID)); err != nil {
		return false, err
	}
	bl.signalPoolConfigCleared(0)
	return true, nil
}

// clearPoolConfig drops the pool from the config (cleanLeaveJoinPool) and confirms it took effect. The services are
// only restarted after a confirmed clear: if the config write failed, a restart would find the pool again, clear
// and restart again — a loop.
func (bl *FxBlockchain) clearPoolConfig(ctx context.Context, poolID int) error {
	bl.cleanLeaveJoinPool(ctx, poolID)
	if bl.getPoolName != nil {
		if pool := bl.getPoolName(); pool != "" && pool != "0" {
			return fmt.Errorf("pool is still %q in the config after clearing it", pool)
		}
	}
	return nil
}
