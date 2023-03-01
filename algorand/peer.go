package algorand

import (
	"bcDB_algorand/Cluster"
	BcGrpc "bcDB_algorand/Proto/blockchain"
	"bcDB_algorand/algorand/common"
	"bytes"
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"strconv"
	"sync"
)

var PeerPools *PeerPool

var LocalPeerPool *PeerPool
var LocalPeer *Peer

var vmu sync.RWMutex // 投票锁
var bmu sync.RWMutex // 区块列表锁
var pmu sync.RWMutex // 提议列表锁

// Peer 对等节点的结构
type Peer struct {
	Algorand      *Algorand
	IncomingVotes map[string]*List       // 投票的map
	Blocks        map[common.Hash]*Block // 提议区块map
	MaxProposals  map[uint64]*Proposal   // 最大提议区块数
}

// NewPeer 新建一个节点
func NewPeer(alg *Algorand) *Peer {
	LocalPeer = &Peer{
		Algorand:      alg,
		IncomingVotes: make(map[string]*List),
		Blocks:        make(map[common.Hash]*Block),
		MaxProposals:  make(map[uint64]*Proposal),
	}
	return LocalPeer
}

// Start 节点启动
func (p *Peer) Start() {
	GetPeerPool().Add(p)
}

// 节点关闭
func (p *Peer) Stop() {
	GetPeerPool().Remove(p)
}

// ID 获得节点的ID
func (p *Peer) ID() string {
	return p.Algorand.Id
}

// Address 获得节点的ip+port
func (p *Peer) Address() string {
	ip := p.Algorand.Id[:len(p.Algorand.Id)-4]
	port := p.Algorand.Id[len(p.Algorand.Id)-4:]
	return fmt.Sprintf("%s:%s", ip, port)
}

// Gossip 广播
func (p *Peer) Gossip(typ int, data []byte) {
	//peers := GetPeerPool().GetPeers()
	for _, peer := range Cluster.LocalNode.Node {
		if peer.IP+strconv.Itoa(peer.Port) == p.ID() {
			continue
		}
		conn, err := grpc.Dial(p.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Println("Network exception!")
			log.Panic(err)
		}
		// 获得grpc句柄
		client := BcGrpc.NewBlockChainServiceClient(conn)
		// 通过句柄调用函数
		_, err = client.Handle(context.Background(), &BcGrpc.TypAndData{
			Typ:  int32(typ),
			Data: data,
		})
		if err != nil {
			log.Panic(err)
		}
		//go peer.Handle(typ, data)
	}
}

// Handle 处理函数
func (p *Peer) Handle(typ int, data []byte) error {
	//log.Println("gossip Handle")
	if typ == BLOCK {
		//log.Println("gossip Handle Block")
		blk := &Block{}
		if err := blk.Deserialize(data); err != nil {
			return err
		}
		p.AddBlock(blk.Hash(), blk)
	} else if typ == BlockProposal {
		//log.Println("gossip Handle BlockProposal")
		bp := &Proposal{}
		if err := bp.Deserialize(data); err != nil {
			return err
		}
		pmu.RLock()
		maxProposal := p.MaxProposals[bp.Round]
		pmu.RUnlock()
		if maxProposal != nil {
			if typ == BlockProposal && bytes.Compare(bp.Prior, maxProposal.Prior) <= 0 {
				return nil
			}
		}
		if err := bp.Verify(p.Algorand.Weight(bp.Address()), ConstructSeed(p.Algorand.SortitionSeed(bp.Round), Role(Proposer, bp.Round, PROPOSE))); err != nil {
			//Log.Errorf("block proposal verification failed, %s", err)
			log.Printf("block proposal verification failed, %s\n", err)
			return err

		}
		p.SetMaxProposal(bp.Round, bp)
	} else if typ == VOTE {
		//log.Println("gossip Handle VOTE")
		vote := &VoteMessage{}
		if err := vote.Deserialize(data); err != nil {
			return err
		}
		key := ConstructVoteKey(vote.Round, vote.Step)
		vmu.RLock()
		list, ok := p.IncomingVotes[key]
		vmu.RUnlock()
		if !ok {
			list = NewList()
		}
		list.Add(vote)
		vmu.Lock()
		p.IncomingVotes[key] = list
		vmu.Unlock()
	}
	return nil
}

// VoteIterator iterator 返回传入消息队列的迭代器。
func (p *Peer) VoteIterator(round uint64, step int) *Iterator {
	key := ConstructVoteKey(round, step)
	vmu.RLock()
	list, ok := p.IncomingVotes[key]
	vmu.RUnlock()
	if !ok {
		list = NewList()
		vmu.Lock()
		p.IncomingVotes[key] = list
		vmu.Unlock()
	}
	return &Iterator{
		list: list,
	}
}

func (p *Peer) GetIncomingMsgs(round uint64, step int) []interface{} {
	vmu.RLock()
	defer vmu.RUnlock()
	l := p.IncomingVotes[ConstructVoteKey(round, step)]
	if l == nil {
		return nil
	}
	return l.List
}

func (p *Peer) GetBlock(hash common.Hash) *Block {
	bmu.RLock()
	defer bmu.RUnlock()
	return p.Blocks[hash]
}

func (p *Peer) AddBlock(hash common.Hash, blk *Block) {
	bmu.Lock()
	defer bmu.Unlock()
	p.Blocks[hash] = blk
}

func (p *Peer) SetMaxProposal(round uint64, proposal *Proposal) {
	pmu.Lock()
	defer pmu.Unlock()
	//log.Infof("node %d set max proposal #%d %s", p.ID(), proposal.Round, proposal.Hash)
	p.MaxProposals[round] = proposal
}

func (p *Peer) GetMaxProposal(round uint64) *Proposal {
	pmu.RLock()
	defer pmu.RUnlock()
	return p.MaxProposals[round]
}

func (p *Peer) ClearProposal(round uint64) {
	pmu.Lock()
	defer pmu.Unlock()
	delete(p.MaxProposals, round)
}

func ConstructVoteKey(round uint64, step int) string {
	return string(bytes.Join([][]byte{
		common.Uint2Bytes(round),
		common.Uint2Bytes(uint64(step)),
	}, nil))
}

var mu sync.RWMutex

type List struct {
	List []interface{}
}

func NewList() *List {
	return &List{}
}

func (l *List) Add(el interface{}) {
	mu.Lock()
	defer mu.Unlock()
	l.List = append(l.List, el)
}

func (l *List) Get(index int) interface{} {
	mu.RLock()
	defer mu.RUnlock()
	if index >= len(l.List) {
		return nil
	}
	return l.List[index]
}

type Iterator struct {
	list  *List
	index int
}

func (it *Iterator) Next() interface{} {
	el := it.list.Get(it.index)
	if el == nil {
		return nil
	}
	it.index++
	return el
}

// PeerPool 节点池
type PeerPool struct {
	mu    sync.Mutex
	peers map[string]*Peer
}

func GetPeerPool() *PeerPool {
	if PeerPools == nil {
		PeerPools = &PeerPool{
			peers: make(map[string]*Peer),
		}
	}
	return PeerPools
}

func (pool *PeerPool) Add(peer *Peer) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.peers[peer.ID()] = peer
}

func (pool *PeerPool) Remove(peer *Peer) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	delete(pool.peers, peer.ID())
}

func (pool *PeerPool) GetPeers() []*Peer {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	var peers []*Peer
	for _, peer := range pool.peers {
		peers = append(peers, peer)
	}
	return peers
}
