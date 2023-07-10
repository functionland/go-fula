package fulamobile

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

var failedPushKeyPrefix = datastore.NewKey("/failed-push")

func (c *Client) markAsFailedPush(ctx context.Context, l ipld.Link) error {
	k := failedPushKeyPrefix.ChildString(l.String())
	return c.ds.Put(ctx, k, nil)
}

func (c *Client) markAsPushedSuccessfully(ctx context.Context, l ipld.Link) error {
	k := failedPushKeyPrefix.ChildString(l.String())
	return c.ds.Delete(ctx, k)
}

func (c *Client) listFailedPushes(ctx context.Context) ([]ipld.Link, error) {
	q := query.Query{
		KeysOnly: true,
		Prefix:   failedPushKeyPrefix.String(),
	}
	results, err := c.ds.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer results.Close()
	var links []ipld.Link
	for r := range results.Next() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if r.Error != nil {
			return nil, r.Error
		}
		key := datastore.RawKey(r.Key)
		c, err := cid.Decode(key.BaseNamespace())
		if err != nil {
			return nil, err
		}
		links = append(links, cidlink.Link{Cid: c})
	}
	return links, nil
}

func (c *Client) listFailedPushesAsString(ctx context.Context) ([]string, error) {
	q := query.Query{
		KeysOnly: true,
		Prefix:   failedPushKeyPrefix.String(),
	}
	results, err := c.ds.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer results.Close()
	var links []string
	for r := range results.Next() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if r.Error != nil {
			return nil, r.Error
		}
		key := datastore.RawKey(r.Key)
		c, err := cid.Decode(key.BaseNamespace())
		if err != nil {
			return nil, err
		}
		links = append(links, c.String())
	}
	return links, nil
}
