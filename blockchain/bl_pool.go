package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

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
		err := bl.StartPingServer(ctx)
		if err != nil {
			return b, err
		}
		if bl.a != nil {
			bl.wg.Add(1)
			go bl.a.AnnounceJoinPoolRequestPeriodically(ctx)
		}
		return b, nil
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
