package GRPC

import (
	"bcDB_algorand/Cluster"
	BcGrpc "bcDB_algorand/Proto/blockchain"
	BCData "bcDB_algorand/blockchain/blockchain_data"
	BCTable "bcDB_algorand/blockchain/blockchain_table"
	"bcDB_algorand/blockqueue"
	"bcDB_algorand/util"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"os"
	"strconv"
	"time"
)

func BlockToGrpcDataBlock(block *BCData.Block) *BcGrpc.DataBlock {
	newGrpcBlock := &BcGrpc.DataBlock{
		ID:                int64(block.ID),
		CurrentBlockHash:  block.CurrentBlockHash,
		PreviousBlockHash: block.PreviousBlockHash,
		MerKelRoot:        block.MerKelRoot,
		TimeTamp:          block.TimeStamp,
	}
	newGrpcBlock.TxInfo = []*BcGrpc.DataTransaction{}
	for _, tx := range block.Transactions {
		newGrpcBlock.TxInfo = append(newGrpcBlock.TxInfo, &BcGrpc.DataTransaction{
			TxID:      tx.TxID,
			DataID:    tx.DataID,
			Table:     tx.Table,
			Key:       tx.Key,
			Value:     tx.Value,
			Possessor: tx.Possessor,
			TimeStamp: tx.TimeStamp,
			PublicKey: tx.PublicKey,
			Signature: tx.Signature,
		})
	}
	return newGrpcBlock
}

func BlockToGrpcTableBlock(block *BCTable.Block) *BcGrpc.TableBlock {
	newGrpcBlock := &BcGrpc.TableBlock{
		ID:                int64(block.ID),
		CurrentBlockHash:  block.CurrentBlockHash,
		PreviousBlockHash: block.PreviousBlockHash,
		MerKelRoot:        block.MerKelRoot,
		TimeTamp:          block.TimeStamp,
	}
	newGrpcBlock.TxInfo = []*BcGrpc.TableTransaction{}
	for _, tx := range block.Transactions {
		newGrpcBlock.TxInfo = append(newGrpcBlock.TxInfo, &BcGrpc.TableTransaction{
			TxID:             tx.TxID,
			Table:            tx.Table,
			PermissionTables: tx.PermissionTable,
			Possessor:        tx.Possessor,
			TimeStamp:        tx.TimeStamp,
			PublicKey:        tx.PublicKey,
			Signature:        tx.Signature,
		})
	}
	return newGrpcBlock
}

func GrpcDataBlockToBlock(block *BcGrpc.DataBlock) *BCData.Block {
	newBlock := &BCData.Block{
		ID:                int(block.ID),
		CurrentBlockHash:  block.CurrentBlockHash,
		PreviousBlockHash: block.PreviousBlockHash,
		MerKelRoot:        block.MerKelRoot,
		TimeStamp:         block.TimeTamp,
	}
	newBlock.Transactions = []*BCData.Transaction{}
	for _, tx := range block.TxInfo {
		newBlock.Transactions = append(newBlock.Transactions, &BCData.Transaction{
			TxID:      tx.TxID,
			DataID:    tx.DataID,
			Table:     tx.Table,
			Key:       tx.Key,
			Value:     tx.Value,
			Possessor: tx.Possessor,
			TimeStamp: tx.TimeStamp,
			PublicKey: tx.PublicKey,
			Signature: tx.Signature,
		})
	}
	return newBlock
}

func GrpcTableBlockToBlock(block *BcGrpc.TableBlock) *BCTable.Block {
	newBlock := &BCTable.Block{
		ID:                int(block.ID),
		CurrentBlockHash:  block.CurrentBlockHash,
		PreviousBlockHash: block.PreviousBlockHash,
		MerKelRoot:        block.MerKelRoot,
		TimeStamp:         block.TimeTamp,
	}
	newBlock.Transactions = []*BCTable.Transaction{}
	for _, tx := range block.TxInfo {
		newBlock.Transactions = append(newBlock.Transactions, &BCTable.Transaction{
			TxID:            tx.TxID,
			Table:           tx.Table,
			PermissionTable: tx.PermissionTables,
			Possessor:       tx.Possessor,
			TimeStamp:       tx.TimeStamp,
			PublicKey:       tx.PublicKey,
			Signature:       tx.Signature,
		})
	}
	return newBlock
}

// GrpcDataTxToDataTx 交易的转换
func GrpcDataTxToDataTx(tx *BcGrpc.DataTransaction) *BCData.Transaction {
	newTx := &BCData.Transaction{
		TxID:      tx.TxID,
		DataID:    tx.DataID,
		Table:     tx.Table,
		Key:       tx.Key,
		Value:     tx.Value,
		Possessor: tx.Possessor,
		TimeStamp: tx.TimeStamp,
		PublicKey: tx.PublicKey,
		Signature: tx.Signature,
	}
	return newTx
}

func GrpcTableTxToTableTx(tx *BcGrpc.TableTransaction) *BCTable.Transaction {
	newTx := &BCTable.Transaction{
		TxID:            tx.TxID,
		Table:           tx.Table,
		PermissionTable: tx.PermissionTables,
		Possessor:       tx.Possessor,
		TimeStamp:       tx.TimeStamp,
		PublicKey:       tx.PublicKey,
		Signature:       tx.Signature,
	}
	return newTx
}

// BroadCast 广播自己已加入集群
func BroadCast(ip string, port int) {
	// 参数为本节点向哪个节点提交的申请
	length := len(Cluster.LocalNode.Node)
	for i := 0; i < length; i++ {
		node := Cluster.LocalNode.Node[i]
		if (node.IP == util.LocalIP && node.Port == util.LocalPort) || (node.IP == ip && node.Port == port) {
			continue
		}
		conn, err := grpc.Dial(fmt.Sprintf("%s:%d", node.IP, node.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Printf("%s:%d 网络异常", node.IP, node.Port)
		}

		// 获得Grpc句柄
		c := BcGrpc.NewBlockChainServiceClient(conn)
		if err != nil {
			log.Panic(err)
		}
		// 通过句柄调用函数
		_, err = c.BroadcastNode(context.Background(), &BcGrpc.NodeInfo{
			LocalIp:   node.IP,
			LocalPort: strconv.Itoa(node.Port),
		})
		if err != nil {
			log.Panic(err)
		}
		conn.Close()
	}
}

// JoinToCluster 加入集群
func JoinToCluster(ip string, port int, key string) error {
	var node Cluster.Node
	fmt.Println(ip)
	// 获得本机的IP和端口号
	node.IP = util.GetLocalIp()
	node.Port = port
	// 获得密钥的hash
	KeyHash := sha256.Sum256([]byte(key))
	Key := KeyHash[:]
	// 提交加入集群的请求(向已加入集群的节点)
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", ip, node.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Println("Network exception!")
		log.Panic(err)
	}
	// 获得grpc句柄
	client := BcGrpc.NewBlockChainServiceClient(conn)
	if err != nil {
		log.Panic(err)
	}
	// 通过句柄调用函数，验证节点是否可以加入集群
	_, err = client.JoinCluster(context.Background(), &BcGrpc.ReqJoin{
		LocalIp:   node.IP,
		LocalPort: strconv.Itoa(node.Port),
		JoinKey:   Key,
	})
	if err != nil {
		log.Panic(err)
	}
	// 判断集群文件是否存在
	_, err = os.Stat("./ClusterInfo")
	if !os.IsNotExist(err) { // 如果没有报错即集群文件存在
		// 如果存在就先清除集群文件
		err = os.Remove("./ClusterInfo")
		if err != nil {
			log.Panic(err)
		}
	}
	// 接收集群文件
	flag := Cluster.Client(ip)
	if !flag {
		return errors.New("接收集群文件失败")
	}
	// 读取集群文件
	clu, err := Cluster.LoadClusterFile("./ClusterInfo")
	if err != nil {
		log.Panic(err)
	}
	// 将自己加入集群
	//clu.AddNodeToClusterFile(&node)
	// 全局变量的覆盖
	Cluster.LocalNode = clu
	return nil
}

// SubmitDataTransaction 交易提交到交易池
func SubmitDataTransaction(tx BCData.Transaction) {
	// 向所有节点发送交易信息
	for _, node := range Cluster.LocalNode.Node {
		if node.IP == util.LocalIP && node.Port == util.LocalPort {
			continue
		}
		conn, err := grpc.Dial(fmt.Sprintf("%s:%d", node.IP, node.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Println("Network exception!")
			log.Panic(err)
		}
		// 获得grpc句柄
		client := BcGrpc.NewBlockChainServiceClient(conn)
		// 通过句柄调用函数
		_, err = client.DataTradingPool(context.Background(), &BcGrpc.DataTransaction{
			TxID:      tx.TxID,
			DataID:    tx.DataID,
			Table:     tx.Table,
			Key:       tx.Key,
			Value:     tx.Value,
			Possessor: tx.Possessor,
			TimeStamp: tx.TimeStamp,
			PublicKey: tx.PublicKey,
			Signature: tx.Signature,
		})
		if err != nil {
			log.Panic(err)
		}
	}
}

// SubmitTableTransaction 交易的提交到交易池
func SubmitTableTransaction(tx BCTable.Transaction) {
	// 向集群节点发送交易信息
	for _, node := range Cluster.LocalNode.Node {
		if node.IP == util.LocalIP && node.Port == util.LocalPort {
			continue
		}
		conn, err := grpc.Dial(fmt.Sprintf("%s:%d", node.IP, node.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Println("Network exception!")
			log.Panic(err)
		}
		// 获得grpc句柄
		client := BcGrpc.NewBlockChainServiceClient(conn)
		// 通过句柄调用函数
		_, err = client.TableTradingPool(context.Background(), &BcGrpc.TableTransaction{
			TxID:             tx.TxID,
			Table:            tx.Table,
			PermissionTables: tx.PermissionTable,
			Possessor:        tx.Possessor,
			TimeStamp:        tx.TimeStamp,
			PublicKey:        tx.PublicKey,
			Signature:        tx.Signature,
		})
		if err != nil {
			log.Panic(err)
		}
	}
}

// DataBlockDistribute 数据区块的分发
func DataBlockDistribute() {
	for {
		time.Sleep(time.Millisecond * 500)
		if blockqueue.LocalDataBlockQueue.Size >= 1 {
			//fmt.Println("RPC数据区块区块的分发")
			get := blockqueue.LocalDataBlockQueue.Get()
			block := get.(BCData.Block)
			for _, node := range Cluster.LocalNode.Node {
				if node.IP == util.LocalIP && node.Port == util.LocalPort {
					continue
				}

				conn, err := grpc.Dial(fmt.Sprintf("%s:%d", node.IP, node.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					fmt.Printf("%s:%d 网络异常", node.IP, node.Port)
				}
				// 获得grpc句柄
				client := BcGrpc.NewBlockChainServiceClient(conn)
				// 调用函数
				_, err = client.DistributeDataBlock(context.Background(), BlockToGrpcDataBlock(&block))
				if err != nil {
					log.Panic(err)
				}
				err = conn.Close()
				if err != nil {
					log.Panic(err)
				}
			}
		}
	}
}

// TableBlockDistribute 权限区块的分发
func TableBlockDistribute() {
	for {
		time.Sleep(time.Second)
		if blockqueue.LocalTableBlockQueue.Size >= 1 {
			log.Println("RPC权限区块区块的分发")
			get := blockqueue.LocalTableBlockQueue.Get()
			block := get.(BCTable.Block)
			for _, node := range Cluster.LocalNode.Node {
				if node.IP == util.LocalIP && node.Port == util.LocalPort {
					continue
				}
				conn, err := grpc.Dial(fmt.Sprintf("%s:%d", node.IP, node.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					fmt.Printf("%s:%d 网络异常", node.IP, node.Port)
				}
				// 获得grpc句柄
				client := BcGrpc.NewBlockChainServiceClient(conn)
				// 调用函数
				_, err = client.DistributeTableBlock(context.Background(), BlockToGrpcTableBlock(&block))
				if err != nil {
					log.Panic(err)
				}
				err = conn.Close()
				if err != nil {
					log.Panic(err)
				}
			}

		}
	}

}

// DataBlockSynchronization 数据区块的同步
func DataBlockSynchronization() {
	time.Sleep(time.Second * 2)
	// 获得本地最后区块的信息
	lastHash := localDataBlockChain.TailHash
	block, err := localDataBlockChain.GetBlockByHash(lastHash)
	if err != nil {
		log.Panic(err)
	}
	// 得到记账节点的IP
	for _, node := range Cluster.LocalNode.Node {
		if node.Account == true {
			// 向记账节点发出申请
			conn, err := grpc.Dial(fmt.Sprintf("%s:%d", node.IP, node.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				fmt.Printf("%s:%d 网络异常", node.IP, node.Port)
			}
			// 获得grpc句柄
			client := BcGrpc.NewBlockChainServiceClient(conn)
			// 通过句柄调用函数
			re, err := client.DataBlockSynchronization(context.Background(), &BcGrpc.ReqDataBlock{
				BlockID: int64(block.ID),
				Hash:    block.CurrentBlockHash,
			})
			// 区块上链
			for i := 0; i < len(re.Blocks); i++ {
				// 区块的转换
				newBlock := GrpcDataBlockToBlock(re.Blocks[len(re.Blocks)-1-i])
				//上链
				localDataBlockChain.AddBlockToChain(*newBlock)
				// 缓存的更新
				localCache.UpdateByDataBlock(*newBlock)
			}
		}
	}
	// 输出信息
	fmt.Println("数据区块同步完成")

}

// TableBlockSynchronization 权限区块的同步
func TableBlockSynchronization() {
	time.Sleep(time.Second * 2)
	// 获得本地最后区块的信息
	lastHash := localTableBlockChain.TailHash
	block, err := localTableBlockChain.GetBlockByHash(lastHash)
	if err != nil {
		log.Panic(err)
	}
	// 得到记账节点的IP
	for _, node := range Cluster.LocalNode.Node {
		if node.Account == true {
			// 向记账节点发出申请
			conn, err := grpc.Dial(fmt.Sprintf("%s:%d", node.IP, node.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				fmt.Printf("%s:%d 网络异常", node.IP, node.Port)
			}
			// 获得grpc句柄
			client := BcGrpc.NewBlockChainServiceClient(conn)
			// 通过句柄调用函数
			re, err := client.TableBlockSynchronization(context.Background(), &BcGrpc.ReqTableBlock{
				BlockID: int64(block.ID),
				Hash:    block.CurrentBlockHash,
			})
			// 区块上链
			for i := 0; i < len(re.Blocks); i++ {
				// 区块的转换
				newBlock := GrpcTableBlockToBlock(re.Blocks[len(re.Blocks)-1-i])
				//上链
				localTableBlockChain.AddBlockToChain(*newBlock)
				// 缓存的更新
				localCache.UpdateByTableBlock(*newBlock)
			}

		}
	}
	fmt.Println("权限区块同步完成")
}
