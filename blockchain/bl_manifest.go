package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

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

func (bl *FxBlockchain) HandleManifestBatchStore(ctx context.Context, poolIDString string, links []ipld.Link) ([]string, error) {
	var linksString []string
	for _, link := range links {
		linksString = append(linksString, link.String())
	}
	poolID, err := strconv.Atoi(poolIDString)
	if err != nil {
		// Handle the error if the conversion fails
		return nil, fmt.Errorf("invalid topic, not an integer: %s", err)
	}
	manifestBatchStoreRequest := ManifestBatchStoreRequest{
		PoolID: poolID,
		Cid:    linksString,
	}

	// Call manifestBatchStore method
	responseBody, err := bl.callBlockchain(ctx, "POST", actionManifestBatchStore, manifestBatchStoreRequest)
	if err != nil {
		return nil, err
	}
	// Interpret the response
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
		return nil, fmt.Errorf("invalid topic, not an integer: %s", err)
	}
	manifestAvailableRequest := ManifestAvailableRequest{
		PoolID: poolID,
	}

	// Call manifestBatchStore method
	responseBody, err := bl.callBlockchain(ctx, "POST", actionManifestAvailable, manifestAvailableRequest)
	if err != nil {
		return nil, err
	}
	// Interpret the response
	var manifestAvailableResponse ManifestAvailableResponse
	if err := json.Unmarshal(responseBody, &manifestAvailableResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifestAvailable response: %w", err)
	}

	var linksWithLimits []LinkWithLimit

	// Group links by uploader
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
