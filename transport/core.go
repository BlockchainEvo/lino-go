// Package transport implements the functionalities that
// directly interact with the blockchain to query data or broadcast transaction.
package transport

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/lino-network/lino-go/errors"
	"github.com/spf13/viper"

	sdk "github.com/cosmos/cosmos-sdk/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	cmn "github.com/tendermint/tmlibs/common"
)

// Transport is a wrapper of tendermint rpc client and codec.
type Transport struct {
	chainId string
	nodeUrl string
	client  rpcclient.Client
	Cdc     *wire.Codec
}

// NewTransportFromConfig initiates an instance of Transport from config files.
func NewTransportFromConfig() *Transport {
	v := viper.New()
	viper.SetConfigType("json")
	v.SetConfigName("config")
	v.AddConfigPath("$HOME/.lino-go/")
	v.AutomaticEnv()
	v.ReadInConfig()

	nodeUrl := v.GetString("node_url")
	if nodeUrl == "" {
		nodeUrl = "localhost:26657"
	}
	rpc := rpcclient.NewHTTP(nodeUrl, "/websocket")
	return &Transport{
		chainId: v.GetString("chain_id"),
		nodeUrl: nodeUrl,
		client:  rpc,
		Cdc:     MakeCodec(),
	}
}

// NewTransportFromArgs initiates an instance of Transport from parameters passed in.
func NewTransportFromArgs(chainID, nodeUrl string) *Transport {
	if nodeUrl == "" {
		nodeUrl = "localhost:46657"
	}
	rpc := rpcclient.NewHTTP(nodeUrl, "/websocket")
	return &Transport{
		chainId: chainID,
		nodeUrl: nodeUrl,
		client:  rpc,
		Cdc:     MakeCodec(),
	}
}

// Query from Tendermint with the provided key and storename
func (t Transport) Query(key cmn.HexBytes, storeName string) (res []byte, err error) {
	return t.query(key, storeName, "key")
}

// Query from Tendermint with the provided subspace and storename
func (t Transport) QuerySubspace(subspace []byte, storeName string) (res []sdk.KVPair, err error) {
	resRaw, err := t.query(subspace, storeName, "subspace")
	if err != nil {
		return res, err
	}
	t.Cdc.UnmarshalJSON(resRaw, &res)
	return
}

func (t Transport) query(key cmn.HexBytes, storeName, endPath string) (res []byte, err error) {
	path := fmt.Sprintf("/store/%s/%s", storeName, endPath)
	node, err := t.GetNode()
	if err != nil {
		return res, err
	}

	result, err := node.ABCIQuery(path, key)
	if err != nil {
		return res, err
	}

	resp := result.Response
	if resp.Code != uint32(0) {
		return res, errors.QueryFail("Query failed").AddBlockChainCode(resp.Code).AddBlockChainLog(resp.Log)
	}

	if resp.Value == nil || len(resp.Value) == 0 {
		return nil, errors.EmptyResponse("Empty response!")
	}
	return resp.Value, nil
}

// QueryBlock queries a block with a certain height from blockchain.
func (t Transport) QueryBlock(height int64) (res *ctypes.ResultBlock, err error) {
	node, err := t.GetNode()
	if err != nil {
		return res, err
	}

	return node.Block(&height)
}

// BroadcastTx broadcasts a transcation to blockchain.
func (t Transport) BroadcastTx(tx []byte) (*ctypes.ResultBroadcastTxCommit, error) {
	node, err := t.GetNode()
	if err != nil {
		return nil, err
	}

	res, err := node.BroadcastTxCommit(tx)
	if err != nil {
		return res, err
	}
	return res, err
}

// SignBuildBroadcast signs msg with private key and then broadcasts
// the transaction to blockchain.
func (t Transport) SignBuildBroadcast(msg interface{},
	privKeyHex string, seq int64) (*ctypes.ResultBroadcastTxCommit, error) {
	privKey, err := GetPrivKeyFromHex(privKeyHex)
	if err != nil {
		return nil, err
	}

	signMsgBytes, err := EncodeSignMsg(t.Cdc, msg, t.chainId, seq)
	if err != nil {
		return nil, err
	}
	// SignatureFromBytes
	sig := privKey.Sign(signMsgBytes)

	// build transaction bytes
	txBytes, err := EncodeTx(t.Cdc, msg, privKey.PubKey(), sig, seq)
	if err != nil {
		return nil, err
	}

	// broadcast
	return t.BroadcastTx(txBytes)
}

// GetNote returns the Tendermint rpc client node.
func (t Transport) GetNode() (rpcclient.Client, error) {
	if t.client == nil {
		return nil, errors.InvalidNodeURL("Must define node URL")
	}
	return t.client, nil
}
