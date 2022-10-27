package poolsdirectory

import (
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p/core"
)

/****
Poolsdirectory is used to create and publish a DAG that holds the Fula pools.
DAG STRUCTURE:

	------------------[HEAD]-------------------------
	|				|		|
	[POOL1]---	-------------[POOL2]		[POOL3]
	|	 |     |				  |
   [PEER1]	 [PEER2]				[PEER3]
****/

/****
Creates a new pool
Gets a name and a unique pool id and returns CID of the head.
If poolid exists in dag, it returns an error.
poolname is optional
****/
func AddPool (poolid String, poolname String) (cid.Cid, error) {

}

/****
Add a bootstrap node for a specific pool
Gets a pool id and ipv4 addresses of a bootstrap node for a specific pool, and returns CID of the head.
Each ipv4 is put under a new node. deduplication happens.
****/
func AddBootstrapnode (poolid String, ipv4 []String) (cid.Cid, error) {

}

/****
Removes a pool
Gets a pool id, and returns CID of the head.
****/
func RemovePool (poolid String) (cid.Cid, error) {

}

/****
Removes the boostrap node for a specific pool id
Gets a pool id and ipv4 addresses of a bootstrap node for a specific pool, and returns CID of the head.
****/
func RemoveBootstrapnode (poolid String, ipv4 []String) (cid.Cid, error) {

}

/****
Published the cid on IPNS
****/
func Publish (root cid.Cid) (String, error) {

}

/****
Get the list of pools
Gets a pool id for a specific pool, and returns the pool details.
****/
func GetPools (poolid String) ([]PoolDetail, error) {

}

/****
Get the boostrap nodes for a specific pool id
Gets a pool id for a specific pool, and returns the ipv4 of the nodes in the pool.
****/
func GetBootstrapnodes (poolid String) ([]String, error) {

}
