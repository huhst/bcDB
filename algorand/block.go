package algorand

import (
	"bcDB_algorand/algorand/common"
	"bytes"
	"encoding/json"
)

type Block struct {
	Round       uint64         `json:"round"`        // 块号，即高度
	ParentHash  common.Hash    `json:"parent_hash"`  // 父区块hash
	Author      common.Address `json:"author"`       // 提议人地址
	AuthorVRF   []byte         `json:"author_vrf"`   // 排序散列
	AuthorProof []byte         `json:"author_proof"` // 排序散列证明
	Time        int64          `json:"time"`         // 块时间戳
	Seed        []byte         `json:"seed"`         // 基于vrf的下一轮种子
	Proof       []byte         `json:"proof"`        // 基于vrf的种子证明
	Data        []byte         `json:"data"`         // 数据字段，模拟事务

	// 不需要hash的部分
	Type      int8   `json:"type"`      // “最终”或“暂定”
	Signature []byte `json:"signature"` // 块的签名

}

func (blk *Block) Hash() common.Hash {
	data := bytes.Join([][]byte{
		common.Uint2Bytes(blk.Round),
		blk.ParentHash.Bytes(),
	}, nil)

	if !blk.Author.Nil() {
		data = append(data, bytes.Join([][]byte{
			blk.Author.Bytes(),
			blk.AuthorVRF,
			blk.AuthorProof,
		}, nil)...)
	}

	if blk.Time != 0 {
		data = append(data, common.Uint2Bytes(uint64(blk.Time))...)
	}

	if blk.Seed != nil {
		data = append(data, blk.Seed...)
	}

	if blk.Proof != nil {
		data = append(data, blk.Proof...)
	}

	return common.Sha256(data)
}

func (blk *Block) RecoverPubkey() *PublicKey {
	return CRecoverPubkey(blk.Signature)
}

func (blk *Block) Serialize() ([]byte, error) {
	return json.Marshal(blk)
}

func (blk *Block) Deserialize(data []byte) error {
	return json.Unmarshal(data, blk)
}
