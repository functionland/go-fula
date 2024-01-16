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
		poolID := r.PoolID
		poolIDStr := strconv.Itoa(poolID)
		requestSubmitted, err := bl.checkIfUserHasOpenPoolRequests(ctx, poolIDStr)
		if err == nil && requestSubmitted {
			err := bl.updatePoolName(poolIDStr)
			if err != nil {
				return []byte("{}"), err
			}
			err = bl.StartPingServer(ctx)
			if err != nil {
				return []byte("{}"), err
			}
			bl.processSuccessfulPoolJoinRequest(ctx, poolIDStr, to)
			return []byte("{}"), nil
		}
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		poolID := r.PoolID
		poolIDStr := strconv.Itoa(poolID)
		requestSubmitted, err := bl.checkIfUserHasOpenPoolRequests(ctx, poolIDStr)
		if err == nil && requestSubmitted {
			err := bl.updatePoolName(poolIDStr)
			if err != nil {
				return []byte("{}"), err
			}
			err = bl.StartPingServer(ctx)
			if err != nil {
				return []byte("{}"), err
			}
			bl.processSuccessfulPoolJoinRequest(ctx, poolIDStr, to)
			return []byte("{}"), nil
		}
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		poolID := r.PoolID
		poolIDStr := strconv.Itoa(poolID)
		err := bl.updatePoolName(poolIDStr)
		if err != nil {
			return b, err
		}
		err = bl.StartPingServer(ctx)
		if err != nil {
			return b, err
		}
		bl.processSuccessfulPoolJoinRequest(ctx, poolIDStr, to)
		return b, nil
	}
}

func (bl *FxBlockchain) processSuccessfulPoolJoinRequest(ctx context.Context, poolIDStr string, to peer.ID) {
	// Create a ticker that triggers every 10 minutes
	err := bl.FetchUsersAndPopulateSets(ctx, poolIDStr, true)
	if err != nil {
		log.Errorw("Error fetching and populating users", "err", err)
	}
	bl.stopFetchUsersAfterJoinChan = make(chan struct{})
	ticker := time.NewTicker(bl.fetchInterval * time.Minute)
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
				if err := bl.FetchUsersAndPopulateSets(ctx, poolIDStr, false); err != nil {
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
		err := bl.StopPingServer(ctx)
		if err != nil {
			return b, err
		}
		if bl.a != nil {
			bl.a.StopJoinPoolRequestAnnouncements()
		}
		// Send a stop signal if the channel is not nil
		if bl.stopFetchUsersAfterJoinChan != nil {
			close(bl.stopFetchUsersAfterJoinChan)
			// Reset the channel to nil to avoid closing a closed channel
			bl.stopFetchUsersAfterJoinChan = nil
		}
		return b, nil
	}
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
	if withMemberListUpdate {
		err := bl.FetchUsersAndPopulateSets(ctx, topicString, false)
		if err != nil {
			return err
		}
	}
	status, exists := bl.GetMemberStatus(from)
	if !exists {
		return fmt.Errorf("peerID does not exist in the list of pool requests or pool members: %s", from)
	}
	if status == common.Pending {
		// Ping
		averageDuration, successCount, err := bl.p.Ping(ctx, from)
		if err != nil {
			log.Errorw("An error occurred during ping", "error", err)
			return err
		}
		vote := averageDuration <= bl.maxPingTime && successCount >= bl.minPingSuccessCount

		log.Debugw("Ping result", "averageDuration", averageDuration, "successCount", successCount, "vote", vote)

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
		responseBody, statusCode, err := bl.callBlockchain(ctx, "POST", actionPoolVote, voteRequest)
		if err != nil {
			return fmt.Errorf("blockchain call error: %w, status code: %d", err, statusCode)
		}

		// Check if the status code is OK; if not, handle it as an error
		if statusCode != http.StatusOK {
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
		log.Infow("Vote cast successfully", "response", voteResponse, "on", from, "by", bl.h.ID())
		// Update member status to unknown
		bl.membersLock.Lock()
		bl.members[from] = common.Unknown
		bl.membersLock.Unlock()
	} else {
		return fmt.Errorf("peerID does not exist in the list of pool requests: %s with status %d", from, status)
	}
	return nil
}
