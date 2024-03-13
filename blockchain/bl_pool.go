package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

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

	//TODO: Ensure it is optimized for long-running calls
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*time.Duration(bl.timeout))
	defer cancel()
	response, statusCode, err := bl.callBlockchain(ctx, method, action, &req)
	if err != nil {
		poolID := req.PoolID
		poolIDStr := strconv.Itoa(poolID)
		requestSubmitted, err := bl.checkIfUserHasOpenPoolRequests(ctx, poolIDStr)
		if err == nil && requestSubmitted {
			errUpdateConfig := bl.updatePoolName(poolIDStr)
			errPingServer := bl.StartPingServer(ctx)
			if errUpdateConfig != nil && errPingServer != nil {
				errMsg := map[string]interface{}{
					"message":     "Pool Join is submitted but Error in Starting Ping Server and updating Config",
					"description": fmt.Sprintf("Error in Ping server: %s , Error in updateConfig: %s", errPingServer.Error(), errUpdateConfig.Error()),
				}
				w.WriteHeader(http.StatusExpectationFailed)
				json.NewEncoder(w).Encode(errMsg)
				return
			} else if errUpdateConfig != nil {
				errMsg := map[string]interface{}{
					"message":     "Pool Join is submitted but Error in updating Config",
					"description": fmt.Sprintf("Error in updateConfig: %s", errUpdateConfig.Error()),
				}
				w.WriteHeader(http.StatusExpectationFailed)
				json.NewEncoder(w).Encode(errMsg)
				return
			} else if errPingServer != nil {
				errMsg := map[string]interface{}{
					"message":     "Pool Join is submitted but Error in Ping Server Start",
					"description": fmt.Sprintf("Error in PingServer: %s", errPingServer.Error()),
				}
				w.WriteHeader(http.StatusExpectationFailed)
				json.NewEncoder(w).Encode(errMsg)
				return
			}
			statusCode = http.StatusAccepted
			err = nil
			response = []byte(fmt.Sprintf("{\"account\":\"\",\"pool_id\":%d}", req.PoolID))
		}
	}
	if statusCode == http.StatusOK {
		statusCode = http.StatusAccepted
	}
	log.Debugw("callblockchain response in JoinPool", "statusCode", statusCode, "response", response, "err", err)
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
	poolIDStr := strconv.Itoa(req.PoolID)
	bl.processSuccessfulPoolJoinRequest(ctx, poolIDStr, bl.h.ID())
	w.WriteHeader(statusCode)
	err1 := json.Unmarshal(response, &res)
	if err1 != nil {
		log.Error("failed to format response: %v", err1)
	}

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

func (bl *FxBlockchain) processSuccessfulPoolJoinRequest(ctx context.Context, poolIDStr string, to peer.ID) {
	// Create a ticker that triggers every 10 minutes
	err := bl.FetchUsersAndPopulateSets(ctx, poolIDStr, true, 15*time.Second)
	if err != nil {
		log.Errorw("Error fetching and populating users", "err", err)
	}
	bl.stopFetchUsersAfterJoinChan = make(chan struct{})
	ticker := time.NewTicker(bl.fetchInterval)
	defer ticker.Stop()
	if bl.wg != nil {
		log.Debug("called wg.Add in PoolJoin ticker")
		bl.wg.Add(1) // Increment the wait group counter
	}
	go func() {
		if bl.wg != nil {
			log.Debug("called wg.Done in PoolJoin ticker")
			defer bl.wg.Done() // Decrement the wait group counter when the goroutine completes
		}
		defer log.Debug("PoolJoin go routine is ending")
		defer ticker.Stop() // Ensure the ticker is stopped when the goroutine exits

		for {
			select {
			case <-ticker.C:
				// Call FetchUsersAndPopulateSets at every tick (10 minutes interval)
				if err := bl.FetchUsersAndPopulateSets(ctx, poolIDStr, false, bl.fetchInterval); err != nil {
					log.Errorw("Error fetching and populating users", "err", err)
				}
				status, exists := bl.GetMemberStatus(to)
				if exists && status == common.Approved {
					ticker.Stop()
					bl.StopPingServer(ctx)
					if bl.a != nil {
						bl.a.StopJoinPoolRequestAnnouncements()
					}
				}
			case <-bl.stopFetchUsersAfterJoinChan:
				// Stop the ticker when receiving a stop signal
				ticker.Stop()
				return
			}
		}
	}()
	// TODO: THIS METHOD BELOW NEEDS TO RE_INITIALIZE ANNONCEMENTS WITH NEW TOPIC ND START IT FIRST
	/*
		if bl.a != nil {
			if bl.wg != nil {
				log.Debug("called wg.Add in PoolJoin ticker2")
				bl.wg.Add(1)
			}
			go func() {
				if bl.wg != nil {
					log.Debug("Called wg.Done in PoolJoin ticker2")
					defer bl.wg.Done() // Decrement the counter when the goroutine completes
				}
				defer log.Debug("PoolJoin ticker2 go routine is ending")

				bl.a.AnnounceJoinPoolRequestPeriodically(ctx)
			}()
		}
	*/
}

func (bl *FxBlockchain) StartPingServer(ctx context.Context) error {
	if bl.p != nil {
		err := bl.p.Start(ctx)
		return err
	}
	return errors.New("ping server cannot be started because it is nil")
}

func (bl *FxBlockchain) StopPingServer(ctx context.Context) error {
	if bl.p != nil {
		err := bl.p.StopServer(ctx)
		return err
	}
	return errors.New("ping server cannot be stopped because it is nil")
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
			errUpdateConfig := bl.updatePoolName("0")
			errPingServer := bl.StopPingServer(ctx)
			if errUpdateConfig != nil && errPingServer != nil {
				errMsg := map[string]interface{}{
					"message":     "Pool Cancel Join is submitted but Error in Stopping Ping Server and updating Config",
					"description": fmt.Sprintf("Error in Ping server: %s , Error in updateConfig: %s", errPingServer.Error(), errUpdateConfig.Error()),
				}
				w.WriteHeader(http.StatusExpectationFailed)
				json.NewEncoder(w).Encode(errMsg)
				return
			} else if errUpdateConfig != nil {
				errMsg := map[string]interface{}{
					"message":     "Pool Cancel Join is submitted but Error in updating Config",
					"description": fmt.Sprintf("Error in updateConfig: %s", errUpdateConfig.Error()),
				}
				w.WriteHeader(http.StatusExpectationFailed)
				json.NewEncoder(w).Encode(errMsg)
				return
			} else if errPingServer != nil {
				errMsg := map[string]interface{}{
					"message":     "Pool Cancel Join is submitted but Error in Ping Server Stop",
					"description": fmt.Sprintf("Error in PingServer: %s", errPingServer.Error()),
				}
				w.WriteHeader(http.StatusExpectationFailed)
				json.NewEncoder(w).Encode(errMsg)
				return
			}
			statusCode = http.StatusAccepted
			err = nil
			response = []byte(fmt.Sprintf("{\"account\":\"\",\"pool_id\":%d}", req.PoolID))
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
	bl.updatePoolName("0")
	bl.StopPingServer(ctx)
	if bl.a != nil {
		bl.a.StopJoinPoolRequestAnnouncements()
	}
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
	// This handles the pending pool requests on a member that is already joined the pool
	if withMemberListUpdate {
		err := bl.FetchUsersAndPopulateSets(ctx, topicString, false, 15*time.Second)
		if err != nil {
			return err
		}
	}
	averageDuration := float64(2000)
	successCount := 0
	status, exists := bl.GetMemberStatus(from)
	if !exists {
		return fmt.Errorf("peerID does not exist in the list of pool requests or pool members: %s", from)
	}
	if status == common.Pending {
		// Ping
		/*
			// Use IPFS Ping
			log.Debugw("****** Pinging pending node", "from", bl.h.ID(), "to", from)
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
		log.Infow("Vote cast successfully", "statusCode", statusCode, "voteResponse", voteResponse, "on", from, "by", bl.h.ID())
		// Update member status to unknown
		bl.membersLock.Lock()
		bl.members[from] = common.Unknown
		bl.membersLock.Unlock()
	} else {
		return fmt.Errorf("peerID does not exist in the list of pool requests: %s with status %d", from, status)
	}
	return nil
}
