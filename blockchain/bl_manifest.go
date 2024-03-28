package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (bl *FxBlockchain) ManifestUpload(ctx context.Context, to peer.ID, r ManifestUploadRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionManifestUpload, &buf)
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

func (bl *FxBlockchain) ManifestStore(ctx context.Context, to peer.ID, r ManifestStoreRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionManifestStore, &buf)
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

func (bl *FxBlockchain) HandleManifestAvailableAllaccountsBatch(ctx context.Context, poolIDString string, links []ipld.Link) ([]ipld.Link, error) {
	var availableLinks []ipld.Link
	poolID, err := strconv.Atoi(poolIDString)
	if err != nil {
		return nil, fmt.Errorf("invalid pool ID: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(bl.timeout))
	defer cancel()

	var cids []string
	for _, link := range links {
		cids = append(cids, link.String())
	}

	reqBody := &AvailableAllaccountsBatchRequest{
		Cids:   cids,
		PoolID: poolID,
	}
	log.Debugw("HandleManifestAvailableAllaccountsBatch", "reqBody", reqBody)

	response, statusCode, err := bl.callBlockchain(ctx, "POST", actionManifestAvailableAllaccountsBatch, reqBody)
	if err != nil {
		return nil, fmt.Errorf("blockchain call failed: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", statusCode)
	}

	var resp BatchManifestAllaccountsResponse
	if err := json.Unmarshal(response, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	log.Debugw("HandleManifestAvailableAllaccountsBatch", "resp", resp)

	// Filter for available manifests.
	for _, manifest := range resp.Manifests {
		c, err := cid.Decode(manifest.Cid)
		if err != nil {
			// Log or handle the error based on your application's logging strategy.
			continue // Skipping invalid CIDs.
		}
		availableLinks = append(availableLinks, cidlink.Link{Cid: c})
	}

	return availableLinks, nil
}

func (bl *FxBlockchain) HandleManifestBatchStore(ctx context.Context, poolIDString string, links []ipld.Link) ([]string, error) {
	var linksString []string
	for _, link := range links {
		linksString = append(linksString, link.String())
	}
	poolID, err := strconv.Atoi(poolIDString)
	if err != nil {
		// Handle the error if the conversion fails
		return nil, fmt.Errorf("invalid poolID, not an integer: %s", err)
	}
	manifestBatchStoreRequest := &ManifestBatchStoreRequest{
		PoolID: poolID,
		Cid:    linksString,
	}

	// Call manifestBatchStore method
	responseBody, statusCode, err := bl.callBlockchain(ctx, "POST", actionManifestBatchStore, manifestBatchStoreRequest)
	if err != nil {
		return nil, fmt.Errorf("blockchain call error: %w, status code: %d", err, statusCode)
	}

	// Check if the status code is OK; if not, handle it as an error
	if statusCode != http.StatusOK {
		var errMsg map[string]interface{}
		if jsonErr := json.Unmarshal(responseBody, &errMsg); jsonErr == nil {
			// If the responseBody is JSON, use it in the error message
			return nil, fmt.Errorf("unexpected response status: %d, message: %s, description: %s",
				statusCode, errMsg["message"], errMsg["description"])
		} else {
			// If the responseBody is not JSON, return it as a plain text error message
			return nil, fmt.Errorf("unexpected response status: %d, body: %s", statusCode, string(responseBody))
		}
	}

	// If the status code is 200, interpret the response
	var manifestBatchStoreResponse ManifestBatchStoreResponse
	if err := json.Unmarshal(responseBody, &manifestBatchStoreResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifestBatchStore response: %w", err)
	}
	return manifestBatchStoreResponse.Cid, nil
}

func (bl *FxBlockchain) HandleManifestsAvailable(ctx context.Context, poolIDString string, limit int) ([]LinkWithLimit, error) {
	poolID, err := strconv.Atoi(poolIDString)
	if err != nil {
		// Handle the error if the conversion fails
		return nil, fmt.Errorf("invalid poolID, not an integer: %s", err)
	}
	manifestAvailableRequest := &ManifestAvailableRequest{
		PoolID: poolID,
	}

	// Call manifestAvailable method
	responseBody, statusCode, err := bl.callBlockchain(ctx, "POST", actionManifestAvailable, manifestAvailableRequest)
	if err != nil {
		return nil, fmt.Errorf("blockchain call error: %w, status code: %d", err, statusCode)
	}

	// Check if the status code is OK; if not, handle it as an error
	if statusCode != http.StatusOK {
		var errMsg map[string]interface{}
		if jsonErr := json.Unmarshal(responseBody, &errMsg); jsonErr == nil {
			// If the responseBody is JSON, use it in the error message
			return nil, fmt.Errorf("unexpected response status: %d, message: %s, description: %s",
				statusCode, errMsg["message"], errMsg["description"])
		} else {
			// If the responseBody is not JSON, return it as a plain text error message
			return nil, fmt.Errorf("unexpected response status: %d, body: %s", statusCode, string(responseBody))
		}
	}

	// If the status code is 200, interpret the response
	var manifestAvailableResponse ManifestAvailableResponse
	if err := json.Unmarshal(responseBody, &manifestAvailableResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifestAvailable response: %w", err)
	}

	var linksWithLimits []LinkWithLimit
	// Group links by uploader and limit the number of links according to the provided limit
	for _, manifest := range manifestAvailableResponse.Manifests {
		c, err := cid.Decode(manifest.ManifestMetadata.Job.Uri)
		if err != nil {
			return nil, fmt.Errorf("failed to decode CID: %w", err)
		}

		if len(linksWithLimits) < limit {
			linksWithLimits = append(linksWithLimits, LinkWithLimit{
				Link:  cidlink.Link{Cid: c},
				Limit: manifest.ReplicationAvailable, // Assuming you have the replication limit in the manifest
			})
		} else {
			break
		}
	}

	return linksWithLimits, nil
}

func (bl *FxBlockchain) ManifestAvailable(ctx context.Context, to peer.ID, r ManifestAvailableRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionManifestAvailable, &buf)
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

func (bl *FxBlockchain) ManifestBatchStore(ctx context.Context, to peer.ID, r ManifestBatchStoreRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionManifestBatchStore, &buf)
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

func (bl *FxBlockchain) ManifestBatchUpload(ctx context.Context, to peer.ID, r ManifestBatchUploadMobileRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionManifestBatchUpload, &buf)
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

func (bl *FxBlockchain) ManifestRemove(ctx context.Context, to peer.ID, r ManifestRemoveRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionManifestRemove, &buf)
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

func (bl *FxBlockchain) ManifestRemoveStorer(ctx context.Context, to peer.ID, r ManifestRemoveStorerRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionManifestRemoveStorer, &buf)
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

func (bl *FxBlockchain) ManifestRemoveStored(ctx context.Context, to peer.ID, r ManifestRemoveStoredRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionManifestRemoveStored, &buf)
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
