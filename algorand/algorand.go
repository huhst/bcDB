package algorand

import (
	"bcDB_algorand/algorand/common"
	"bcDB_algorand/blockchain/blockchain_data"
	"bcDB_algorand/blockchain/blockchain_table"
	"bcDB_algorand/blockqueue"
	"bytes"
	"context"
	"errors"
	"github.com/rcrowley/go-metrics"
	"log"
	"math/big"
	"math/rand"
	"sync"
	"time"
)

// 常量
var (
	dataIsDone           bool  = true
	tableIsDone          bool  = true
	Type                 uint8 // 标识区块的类型 0:数据区块，1:权限区块
	ErrCountVotesTimeout       = errors.New("count votes timeout")
	/*全局变量*/

	// MetricsRound 轮次
	MetricsRound uint64 = 1
	// ProposerSelectedCounter 提议者选择的计数器
	ProposerSelectedCounter = metrics.NewRegisteredCounter("blockproposal/subusers/count", nil)
	// ProposerSelectedHistogram 提议者选择的分布
	ProposerSelectedHistogram = metrics.NewRegisteredHistogram("blockproposal/subusers", nil, metrics.NewUniformSample(1028))
)

// Algorand Algorand的结构
type Algorand struct {
	Id          string      // 进程的id（ip+port）
	Privkey     *PrivateKey // 私钥
	Pubkey      *PublicKey  // 公钥
	DataChain   *Blockchain // 数据链
	TableChain  *Blockchain // 权限链
	Peer        *Peer
	QuitCh      chan struct{} // 退出通道
	HangForever chan struct{} //挂起通道
}

// NewAlgorand 新建一个Algorand
func NewAlgorand(id string) *Algorand {
	// 得到随机种子
	rand.Seed(time.Now().UnixNano())
	// 获取公私钥
	pub, priv, _ := NewKeyPair()
	alg := &Algorand{
		Id:         id,
		Privkey:    priv,
		Pubkey:     pub,
		DataChain:  NewDataBlockchain(),
		TableChain: NewTableBlockchain(),
	}
	// 建立新的对等节点alg
	alg.Peer = NewPeer(alg)
	return alg
}

func (alg *Algorand) Start() {
	//fmt.Println("启动algorand")
	alg.QuitCh = make(chan struct{})
	alg.HangForever = make(chan struct{})
	alg.Peer.Start()
	go alg.Run()
}

func (alg *Algorand) Stop() {
	close(alg.QuitCh)
	close(alg.HangForever)
	alg.Peer.Stop()
}

// Round 返回最新的整数。
func (alg *Algorand) Round() uint64 {
	return alg.LastBlock().Round
}

func (alg *Algorand) LastBlock() *Block {
	if Type == 0 {
		return alg.DataChain.Last
	} else {
		return alg.TableChain.Last
	}

}

// Weight 返回给定地址的权重。
func (alg *Algorand) Weight(address common.Address) uint64 {
	return TokenPerUser
}

// tokenOwn 返回自身节点拥有的令牌数量（权重）。
func (alg *Algorand) tokenOwn() uint64 {
	return alg.Weight(alg.Address())
}

// VrfSeed seed 返回块r的基于vrf的种子。
func (alg *Algorand) VrfSeed(round uint64) (seed, proof []byte, err error) {
	if round == 0 {
		if Type == 0 {
			return alg.DataChain.Genesis.Seed, nil, nil
		} else {
			return alg.TableChain.Genesis.Seed, nil, nil
		}
	}
	// 获得最后的区块
	var lastBlock *Block
	if Type == 0 {
		lastBlock = alg.DataChain.GetByRound(round - 1)
	} else {
		lastBlock = alg.TableChain.GetByRound(round - 1)
	}
	// 最后一个区块不是genesis区块，验证种子r-1。
	if round != 1 {
		var lastParentBlock *Block
		if Type == 0 {
			lastParentBlock = alg.DataChain.Get(lastBlock.ParentHash, lastBlock.Round-1)
		} else {
			lastParentBlock = alg.TableChain.Get(lastBlock.ParentHash, lastBlock.Round-1)
		}
		if lastBlock.Proof != nil {
			// vrf-based seed
			pubkey := CRecoverPubkey(lastBlock.Signature)
			m := bytes.Join([][]byte{lastParentBlock.Seed, common.Uint2Bytes(lastBlock.Round)}, nil)
			err = pubkey.VerifyVRF(lastBlock.Proof, m)
		} else if bytes.Compare(lastBlock.Seed, common.Sha256(
			bytes.Join([][]byte{
				lastParentBlock.Seed,
				common.Uint2Bytes(lastBlock.Round)},
				nil)).Bytes()) != 0 {
			// hash-based seed
			err = errors.New("hash seed invalid")
		}
		if err != nil {
			// seed r-1 invalid
			return common.Sha256(bytes.Join([][]byte{lastBlock.Seed, common.Uint2Bytes(lastBlock.Round + 1)}, nil)).Bytes(), nil, nil
		}
	}

	seed, proof, err = alg.Privkey.Evaluate(bytes.Join([][]byte{lastBlock.Seed, common.Uint2Bytes(lastBlock.Round + 1)}, nil))
	return
}

// EmptyBlock 空区块
func (alg *Algorand) EmptyBlock(round uint64, prevHash common.Hash) *Block {
	return &Block{
		Round:      round,
		ParentHash: prevHash,
	}
}

// SortitionSeed 返回具有刷新间隔R的选择种子。
func (alg *Algorand) SortitionSeed(round uint64) []byte {
	realR := round - 1
	mod := round % R
	if realR < mod {
		realR = 0
	} else {
		realR -= mod
	}
	if Type == 0 {
		return alg.DataChain.GetByRound(realR).Seed
	} else {
		return alg.TableChain.GetByRound(realR).Seed
	}
}

func (alg *Algorand) Address() common.Address {
	return common.BytesToAddress(alg.Pubkey.Bytes())
}

// Run 在无限循环中执行Algorand算法的所有过程。
func (alg *Algorand) Run() {
	// 为所有对等方准备好睡眠1毫秒。2
	time.Sleep(1 * time.Millisecond)
	for {
		time.Sleep(1 * time.Millisecond)
		// todo 需要判断是否打包了新的区块
		if blockqueue.DataBlock2Algorand.Size >= 1 && dataIsDone {
			get := blockqueue.DataBlock2Algorand.Get()
			bcBlock := get.(blockchain_data.Block)
			// 执行algorand算法
			go alg.ProcessMain(&bcBlock, nil)
		}
		if blockqueue.TableBlock2Algorand.Size >= 1 && tableIsDone {
			get := blockqueue.TableBlock2Algorand.Get()
			bcBlock := get.(blockchain_table.Block)
			// 执行algorand算法
			go alg.ProcessMain(nil, &bcBlock)
		}
	}

}

// ProcessMain 执行algorand算法的主要处理。
func (alg *Algorand) ProcessMain(bcDataBlock *blockchain_data.Block, bcTableBlock *blockchain_table.Block) {
	dataIsDone = false
	tableIsDone = false
	if bcDataBlock != nil {
		Type = 0
	} else {
		Type = 1
	}
	if MetricsRound == alg.Round() {
		ProposerSelectedHistogram.Update(ProposerSelectedCounter.Count())
		ProposerSelectedCounter.Clear()
		MetricsRound = alg.Round() + 1
	}
	currRound := alg.Round() + 1
	var block *Block
	if Type == 0 {
		// 1. 区块提议
		//log.Println("数据区块提议")
		block = alg.BlockProposal(bcDataBlock, nil)
	} else {
		// 1. 区块提议
		//log.Println("权限区块提议")
		block = alg.BlockProposal(nil, bcTableBlock)
	}
	log.Printf("node %s init BA with block #%d %s, is empty? %v\n", alg.Id, block.Round, block.Hash(), block.Signature == nil)

	// 2. 用具有最高优先级的block初始化BA。
	consensusType, block := alg.BA(currRound, block)

	// 3. 就最终或暂定新区块达成共识。
	log.Printf("node %s reach consensus %d at Round %d, block hash %s, is empty? %v\n", alg.Id, consensusType, currRound, block.Hash(), block.Signature == nil)

	// 4. 上链
	if Type == 0 {
		log.Println("数据区块上链")
		blockqueue.Algorand2DataBlock.Put(block)
		alg.DataChain.Add(block)
	} else {
		log.Println("权限区块上链")
		blockqueue.Algorand2TableBlock.Put(block)
		alg.TableChain.Add(block)
	}
	dataIsDone = true
	tableIsDone = true
	// TODO: 5. clear cache
}

// ProposeBlock 提出一个新的块
func (alg *Algorand) ProposeBlock(bcDataBlock *blockchain_data.Block, bcTableBlock *blockchain_table.Block) *Block {
	var Data []byte
	currRound := alg.Round() + 1
	seed, proof, err := alg.VrfSeed(currRound)
	if err != nil {
		return alg.EmptyBlock(currRound, alg.LastBlock().Hash())
	}
	if Type == 0 {
		//数据为传入区块的hash
		Data = bcDataBlock.CurrentBlockHash
	} else {
		//数据为传入区块的hash
		Data = bcTableBlock.CurrentBlockHash
	}
	blk := &Block{
		Round:      currRound,
		Seed:       seed,
		ParentHash: alg.LastBlock().Hash(),
		Author:     alg.Pubkey.Address(),
		Time:       time.Now().Unix(),
		Proof:      proof,
		Data:       Data,
	}
	bhash := blk.Hash()
	sign, _ := alg.Privkey.Sign(bhash.Bytes())
	blk.Signature = sign
	log.Printf("node %s propose a new block #%d %s\n", alg.Id, blk.Round, blk.Hash())
	return blk
}

// BlockProposal 执行块提议过程。
func (alg *Algorand) BlockProposal(bcDataBlock *blockchain_data.Block, bcTableBlock *blockchain_table.Block) *Block {
	round := alg.Round() + 1
	vrf, proof, subusers := alg.Sortition(alg.SortitionSeed(round), Role(Proposer, round, PROPOSE), ExpectedBlockProposers, alg.tokenOwn())
	// 已选择
	//fmt.Println("subusers:", subusers)
	if subusers > 0 {
		ProposerSelectedCounter.Inc(1)
		var (
			newBlk       *Block
			proposalType int
		)
		// 区块的提议，判断是数据区块还是权限区块
		if bcDataBlock != nil {
			newBlk = alg.ProposeBlock(bcDataBlock, nil)
			proposalType = BlockProposal
		} else {
			newBlk = alg.ProposeBlock(nil, bcTableBlock)
			proposalType = BlockProposal
		}

		proposal := &Proposal{
			Round:  newBlk.Round,
			Hash:   newBlk.Hash(),
			Prior:  MaxPriority(vrf, subusers),
			VRF:    vrf,
			Proof:  proof,
			Pubkey: alg.Pubkey.Bytes(),
		}
		alg.Peer.SetMaxProposal(round, proposal)
		alg.Peer.AddBlock(newBlk.Hash(), newBlk)
		blkMsg, _ := newBlk.Serialize()
		proposalMsg, _ := proposal.Serialize()

		// 广播给所有对等节点
		alg.Peer.Gossip(BLOCK, blkMsg)
		alg.Peer.Gossip(proposalType, proposalMsg)

	}

	//等待 λstepvar+λpriority 时间以确定最高优先级。
	timeoutForPriority := time.NewTimer(LamdaStepvar + LamdaPriority)
	<-timeoutForPriority.C

	// block提议超时
	timeoutForBlockFlying := time.NewTimer(LamdaBlock)
	ticker := time.NewTicker(200 * time.Millisecond)
	for {
		select {
		case <-timeoutForBlockFlying.C:
			// 空的block
			return alg.EmptyBlock(round, alg.LastBlock().Hash())
		case <-ticker.C:
			// 获取具有最高优先级的块
			pp := alg.Peer.GetMaxProposal(round)
			if pp == nil {
				continue
			}
			blk := alg.Peer.GetBlock(pp.Hash)
			if blk != nil {
				return blk
			}
		}
	}
}

// Sortition 运行加密选择过程并返回vrf、证明和所选子用户的数量。
func (alg *Algorand) Sortition(seed, role []byte, expectedNum int, weight uint64) (vrf, proof []byte, selected int) {
	vrf, proof, _ = alg.Privkey.Evaluate(ConstructSeed(seed, role))
	selected = SubUsers(expectedNum, weight, vrf)
	return
}

// VerifySort 验证vrf并返回所选子用户的数量。
func (alg *Algorand) VerifySort(vrf, proof, seed, role []byte, expectedNum int) int {
	if err := alg.Pubkey.VerifyVRF(proof, ConstructSeed(seed, role)); err != nil {
		return 0
	}

	return SubUsers(expectedNum, alg.tokenOwn(), vrf)
}

// CommitteeVote 投票支持`value`
func (alg *Algorand) CommitteeVote(round uint64, step int, expectedNum int, hash common.Hash) error {

	vrf, proof, j := alg.Sortition(alg.SortitionSeed(round), Role(Committee, round, step), expectedNum, alg.tokenOwn())
	//log.Println("CommitteeVote j:", j)
	if j > 0 {
		var parentHash common.Hash
		if Type == 0 {
			parentHash = alg.DataChain.Last.Hash()
		} else {
			parentHash = alg.TableChain.Last.Hash()
		}
		// 广播投票信息
		voteMsg := &VoteMessage{
			Round:      round,
			Step:       step,
			VRF:        vrf,
			Proof:      proof,
			ParentHash: parentHash,
			Hash:       hash,
		}
		// 投票信息的签名
		_, err := voteMsg.Sign(alg.Privkey)
		if err != nil {
			return err
		}
		// 投票信息的序列化
		data, err := voteMsg.Serialize()
		if err != nil {
			return err
		}
		alg.Peer.Gossip(VOTE, data)
	}
	return nil
}

// BA 在下一轮次中，用一个提议的区块运行BA*共识
func (alg *Algorand) BA(round uint64, block *Block) (int8, *Block) {
	//log.Println("join BA")
	var (
		newBlk *Block
		hash   common.Hash
	)
	hash = alg.Reduction(round, block.Hash())
	//log.Println("Reduction end!")
	hash = alg.BinaryBA(round, hash)
	//log.Println("BinaryBA end!")
	r, _ := alg.CountVotes(round, FINAL, FinalThreshold, ExpectedFinalCommitteeMembers, LamdaStep)
	//log.Println("CountVotes end!")
	if prevHash := alg.LastBlock().Hash(); hash == EmptyHash(round, prevHash) {
		// empty block
		newBlk = alg.EmptyBlock(round, prevHash)
	} else {
		newBlk = alg.Peer.GetBlock(hash)
	}
	if r == hash {
		newBlk.Type = FinalConsensus
		return FinalConsensus, newBlk
	} else {
		newBlk.Type = TentativeConsensus
		return TentativeConsensus, newBlk
	}
}

// Reduction 第二步 Reduction.
func (alg *Algorand) Reduction(round uint64, hash common.Hash) common.Hash {
	//log.Println("join Reduction")
	// step 1: 广播block的hash
	alg.CommitteeVote(round, ReductionOne, ExpectedCommitteeMembers, hash)

	// 其他用户可能仍在等待块建议
	// 设置等待时间为 λ block + λ step
	hash1, err := alg.CountVotes(round, ReductionOne, ThresholdOfBAStep, ExpectedCommitteeMembers, LamdaBlock+LamdaStep)

	// step 2: 重新广播block的hash
	var empty common.Hash
	if Type == 0 {
		empty = EmptyHash(round, alg.DataChain.Last.Hash())
	} else {
		empty = EmptyHash(round, alg.TableChain.Last.Hash())
	}

	if err == ErrCountVotesTimeout {
		alg.CommitteeVote(round, ReductionTwo, ExpectedCommitteeMembers, empty)
	} else {
		alg.CommitteeVote(round, ReductionTwo, ExpectedCommitteeMembers, hash1)
	}

	hash2, err := alg.CountVotes(round, ReductionTwo, ThresholdOfBAStep, ExpectedCommitteeMembers, LamdaStep)
	if err == ErrCountVotesTimeout {
		return empty
	}
	return hash2
}

// BinaryBA 执行，直到对给定的“hash”或“empty_hash”达成共识。
func (alg *Algorand) BinaryBA(round uint64, hash common.Hash) common.Hash {
	//log.Println("join Binary BA")
	var (
		step = 1
		r    = hash
		err  error
	)
	var empty common.Hash
	if Type == 0 {
		empty = EmptyHash(round, alg.DataChain.Last.Hash())
	} else {
		empty = EmptyHash(round, alg.TableChain.Last.Hash())
	}

	defer func() {
		log.Printf("node %s complete BinaryBA with %d steps\n", alg.Id, step)
	}()
	for step < MAXSTEPS {
		//log.Println("BinaryBA step:", step)
		alg.CommitteeVote(round, step, ExpectedCommitteeMembers, r)
		r, err = alg.CountVotes(round, step, ThresholdOfBAStep, ExpectedCommitteeMembers, LamdaStep)
		if err == ErrCountVotesTimeout {
			r = hash
		} else if r != empty {
			for s := step + 1; s <= step+3; s++ {
				alg.CommitteeVote(round, s, ExpectedCommitteeMembers, r)
			}
			if step == 1 {
				alg.CommitteeVote(round, FINAL, ExpectedFinalCommitteeMembers, r)
			}
			return r
		}
		step++

		alg.CommitteeVote(round, step, ExpectedCommitteeMembers, r)
		r, err = alg.CountVotes(round, step, ThresholdOfBAStep, ExpectedCommitteeMembers, LamdaStep)
		if err == ErrCountVotesTimeout {
			r = empty
		} else if r == empty {
			for s := step + 1; s <= step+3; s++ {
				alg.CommitteeVote(round, s, ExpectedCommitteeMembers, r)
			}
			return r
		}
		step++

		alg.CommitteeVote(round, step, ExpectedCommitteeMembers, r)
		r, err = alg.CountVotes(round, step, ThresholdOfBAStep, ExpectedCommitteeMembers, LamdaStep)
		if err == ErrCountVotesTimeout {
			if alg.CommonCoin(round, step, ExpectedCommitteeMembers) == 0 {
				r = hash
			} else {
				r = empty
			}
		}
	}
	log.Println("reach the maxStep, hang forever")
	// hang forever
	<-alg.HangForever
	return common.Hash{}
}

// CountVotes 计算轮次和步数的选票
func (alg *Algorand) CountVotes(round uint64, step int, threshold float64, expectedNum int, timeout time.Duration) (common.Hash, error) {
	expired := time.NewTimer(timeout)
	counts := make(map[common.Hash]int)
	voters := make(map[string]struct{})
	it := alg.Peer.VoteIterator(round, step)
	for {
		msg := it.Next()
		if msg == nil {
			select {
			case <-expired.C:
				// timeout
				return common.Hash{}, ErrCountVotesTimeout
			default:
			}
		} else {
			voteMsg := msg.(*VoteMessage)
			votes, hash, _ := alg.ProcessMsg(msg.(*VoteMessage), expectedNum)
			pubkey := voteMsg.RecoverPubkey()
			if _, exist := voters[string(pubkey.Pk)]; exist || votes == 0 {
				continue
			}
			voters[string(pubkey.Pk)] = struct{}{}
			counts[hash] += votes
			// if we got enough votes, then output the target hash
			//log.Infof("node %d receive votes %v,threshold %v at step %d", alg.id, counts[hash], uint64(float64(expectedNum)*threshold), step)
			if uint64(counts[hash]) >= uint64(float64(expectedNum)*threshold) {
				return hash, nil
			}
		}
	}
}

// ProcessMsg 验证传入的投票消息。
func (alg *Algorand) ProcessMsg(message *VoteMessage, expectedNum int) (votes int, hash common.Hash, vrf []byte) {
	if err := message.VerifySign(); err != nil {
		return 0, common.Hash{}, nil
	}

	// discard messages that do not extend this chain
	prevHash := message.ParentHash
	if Type == 0 {
		if prevHash != alg.DataChain.Last.Hash() {
			return 0, common.Hash{}, nil
		}
	} else {
		if prevHash != alg.TableChain.Last.Hash() {
			return 0, common.Hash{}, nil
		}
	}

	votes = alg.VerifySort(message.VRF, message.Proof, alg.SortitionSeed(message.Round), Role(Committee, message.Round, message.Step), expectedNum)
	hash = message.Hash
	vrf = message.VRF
	return
}

// CommonCoin 计算所有用户共用的硬币。
// 如果对手向网络发送错误消息并阻止网络达成共识，这是一种帮助Algorand恢复的程序。
func (alg *Algorand) CommonCoin(round uint64, step int, expectedNum int) int64 {
	minhash := new(big.Int).Exp(big.NewInt(2), big.NewInt(common.HashLength), big.NewInt(0))
	msgList := alg.Peer.GetIncomingMsgs(round, step)
	for _, m := range msgList {
		msg := m.(*VoteMessage)
		votes, _, vrf := alg.ProcessMsg(msg, expectedNum)
		for j := 1; j < votes; j++ {
			h := new(big.Int).SetBytes(common.Sha256(bytes.Join([][]byte{vrf, common.Uint2Bytes(uint64(j))}, nil)).Bytes())
			if h.Cmp(minhash) < 0 {
				minhash = h
			}
		}
	}
	return minhash.Mod(minhash, big.NewInt(2)).Int64()
}

// Role 返回当前回合和步骤中的role的字节
func Role(iden string, round uint64, step int) []byte {
	return bytes.Join([][]byte{
		[]byte(iden),
		common.Uint2Bytes(round),
		common.Uint2Bytes(uint64(step)),
	}, nil)
}

// MaxPriority 返回最高优先级的提议块
func MaxPriority(vrf []byte, users int) []byte {
	var maxPrior []byte
	for i := 1; i <= users; i++ {
		prior := common.Sha256(bytes.Join([][]byte{vrf, common.Uint2Bytes(uint64(i))}, nil)).Bytes()
		if bytes.Compare(prior, maxPrior) > 0 {
			maxPrior = prior
		}
	}
	return maxPrior
}

// SubUsers 返回根据数学协议确定的所选“子用户”数量
func SubUsers(expectedNum int, weight uint64, vrf []byte) int {
	//binomial := NewBinomial(int64(weight), int64(expectedNum), int64(TotalTokenAmount()))
	binomial := NewApproxBinomial(int64(expectedNum), weight)
	//binomial := &distuv.Binomial{
	//	N: float64(Weight),
	//	P: float64(expectedNum) / float64(TotalTokenAmount()),
	//}
	// hash / 2^hashlen ∉ [ ∑0,j B(k;w,p), ∑0,j+1 B(k;w,p))
	hashBig := new(big.Int).SetBytes(vrf)
	maxHash := new(big.Int).Exp(big.NewInt(2), big.NewInt(common.HashLength*8), nil)
	hash := new(big.Rat).SetFrac(hashBig, maxHash)
	var lower, upper *big.Rat
	j := 0
	for uint64(j) <= weight {
		if upper != nil {
			lower = upper
		} else {
			lower = binomial.CDF(int64(j))
		}
		upper = binomial.CDF(int64(j + 1))
		//log.Infof("hash %v, lower %v , upper %v", hash.Sign(), lower.Sign(), upper.Sign())
		if hash.Cmp(lower) >= 0 && hash.Cmp(upper) < 0 {
			break
		}
		j++
	}
	if uint64(j) > weight {
		j = 0
	}
	//j := ParallelTrevels(runtime.NumCPU(), Weight, hash, binomial)
	return j
}

func ParallelTrevels(core int, N uint64, hash *big.Rat, binomial Binomial) int {
	var wg sync.WaitGroup
	groups := N / uint64(core)
	background, cancel := context.WithCancel(context.Background())
	resChan := make(chan int)
	notFound := make(chan struct{})
	for i := 0; i < core; i++ {
		go func(ctx context.Context, begin uint64) {
			wg.Add(1)
			defer wg.Done()
			var (
				end          uint64
				upper, lower *big.Rat
			)
			if begin == uint64(core-2) {
				end = N + 1
			} else {
				end = groups * (begin + 1)
			}
			for j := groups * begin; j < end; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if upper != nil {
					lower = upper
				} else {
					lower = binomial.CDF(int64(j))
				}
				upper = binomial.CDF(int64(j + 1))
				//log.Infof("hash %v, lower %v , upper %v", hash.Sign(), lower.Sign(), upper.Sign())
				if hash.Cmp(lower) >= 0 && hash.Cmp(upper) < 0 {
					resChan <- int(j)
					return
				}
				j++
			}
			return
		}(background, uint64(i))
	}

	go func() {
		wg.Wait()
		close(notFound)
	}()

	select {
	case j := <-resChan:
		cancel()
		return j
	case <-notFound:
		//cancel()
		return 0
	}
}

// ConstructSeed 为vrf生成构造一个新的字节
func ConstructSeed(seed, role []byte) []byte {
	return bytes.Join([][]byte{seed, role}, nil)
}

func EmptyHash(round uint64, prev common.Hash) common.Hash {
	return common.Sha256(bytes.Join([][]byte{
		common.Uint2Bytes(round),
		prev.Bytes(),
	}, nil))
}
