package algorand

import (
	"bcDB_algorand/algorand/common"
	"bytes"
	"encoding/json"
	"errors"
)

// 信息类型常量
const (
	VOTE          = iota
	BlockProposal // 区块的提议
	BLOCK         // 区块
)

// VoteMessage 投票信息的结构体
type VoteMessage struct {
	Signature  []byte      `json:"signature"`
	Round      uint64      `json:"Round"`
	Step       int         `json:"step"`
	VRF        []byte      `json:"vrf"`
	Proof      []byte      `json:"proof"`
	ParentHash common.Hash `json:"parentHash"`
	Hash       common.Hash `json:"hash"`
}

// Serialize 使用JSON进行序列化
func (v *VoteMessage) Serialize() ([]byte, error) {
	return json.Marshal(v)
}

// Deserialize 使用JSON进行反序列化
func (v *VoteMessage) Deserialize(data []byte) error {
	return json.Unmarshal(data, v)
}

// VerifySign 验证签名
func (v *VoteMessage) VerifySign() error {
	pubkey := v.RecoverPubkey()
	data := bytes.Join([][]byte{
		common.Uint2Bytes(v.Round),
		common.Uint2Bytes(uint64(v.Step)),
		v.VRF,
		v.Proof,
		v.ParentHash.Bytes(),
		v.Hash.Bytes(),
	}, nil)
	return pubkey.VerifySign(data, v.Signature)
}

// Sign 签名
func (v *VoteMessage) Sign(priv *PrivateKey) ([]byte, error) {
	data := bytes.Join([][]byte{
		common.Uint2Bytes(v.Round),
		common.Uint2Bytes(uint64(v.Step)),
		v.VRF,
		v.Proof,
		v.ParentHash.Bytes(),
		v.Hash.Bytes(),
	}, nil)
	sign, err := priv.Sign(data)
	if err != nil {
		return nil, err
	}
	v.Signature = sign
	return sign, nil
}

// CRecoverPubkey 恢复公钥
func (v *VoteMessage) RecoverPubkey() *PublicKey {
	return CRecoverPubkey(v.Signature)
}

// Proposal 定义提议的结构体
type Proposal struct {
	Round  uint64      `json:"Round"`      // 轮次
	Hash   common.Hash `json:"hash"`       // hash值
	Prior  []byte      `json:"prior"`      // 提议者
	VRF    []byte      `json:"vrf"`        // vrf of user's Sortition hash
	Proof  []byte      `json:"proof"`      // 证明
	Pubkey []byte      `json:"public_key"` // 公钥
}

// Serialize 序列化
func (b *Proposal) Serialize() ([]byte, error) {
	return json.Marshal(b)
}

// Deserialize 反序列化
func (b *Proposal) Deserialize(data []byte) error {
	return json.Unmarshal(data, b)
}

// PublicKey 得到公钥
func (b *Proposal) PublicKey() *PublicKey {
	return &PublicKey{b.Pubkey}
}

// Address 得到提议人的地址
func (b *Proposal) Address() common.Address {
	return common.BytesToAddress(b.Pubkey) // 通过公钥得到地址
}

// Verify 验证
func (b *Proposal) Verify(weight uint64, m []byte) error {
	// 验证vrf
	pubkey := b.PublicKey()
	if err := pubkey.VerifyVRF(b.Proof, m); err != nil {
		return err
	}

	// 验证优先级
	subusers := SubUsers(ExpectedBlockProposers, weight, b.VRF)
	if bytes.Compare(MaxPriority(b.VRF, subusers), b.Prior) != 0 {
		return errors.New("max priority mismatch")
	}

	return nil
}
