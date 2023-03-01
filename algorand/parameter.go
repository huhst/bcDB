// 保存参数的文件

package algorand

import "time"

var (
	UserAmount   uint64 = 100
	TokenPerUser uint64 = 10000
	//Malicious      uint64 = 0
	//NetworkLatency = 0
)

func TotalTokenAmount() uint64 { return UserAmount * TokenPerUser }

const (
	// Algorand 系统参数 参考文档的
	ExpectedBlockProposers        = 26    // 期望区块提议者数量   26
	ExpectedCommitteeMembers      = 10    // 期望委员会成员  10
	ThresholdOfBAStep             = 0.685 // BA步长阈值
	ExpectedFinalCommitteeMembers = 20    // 期望最终委员会成员  20
	FinalThreshold                = 0.67  // 最终阈值  // 0.74
	MAXSTEPS                      = 3     // 最长步长

	// 超时参数
	//LamdaPriority = 5 * time.Second  // time to Gossip Sortition proofs.
	//LamdaBlock    = 1 * time.Minute  // 接收块超时。
	//LamdaStep     = 20 * time.Second // BA*步骤超时。
	//LamdaStepvar  = 5 * time.Second  //BA*完成时间差异的估计。

	LamdaPriority = 2 * time.Second // time to Gossip Sortition proofs.
	LamdaBlock    = 1 * time.Second // 接收块超时。
	LamdaStep     = 2 * time.Second // BA*步骤超时。
	LamdaStepvar  = 2 * time.Second //BA*完成时间差异的估计。

	// interval
	R = 1000 // seed 刷新间隔 (# of rounds)

	// 辅助常量
	Committee = "Committee" // 委员会成员
	Proposer  = "proposer"  //提议者

	// step
	PROPOSE      = 1000
	ReductionOne = 1001
	ReductionTwo = 1002
	FINAL        = 1003

	FinalConsensus     = 0
	TentativeConsensus = 1
)
