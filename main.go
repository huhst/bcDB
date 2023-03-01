package main

import (
	"bcDB_algorand/Cluster"
	"bcDB_algorand/algorand"
	"bcDB_algorand/blockqueue"
	"bcDB_algorand/client"
	"bcDB_algorand/server"
	"bcDB_algorand/serverExec"
	"bcDB_algorand/util"
	"fmt"
	"log"
	"os"
	"strconv"
)

func main() {

	blockqueue.LocalDataBlockQueue = blockqueue.Init()
	blockqueue.LocalTableBlockQueue = blockqueue.Init()
	blockqueue.DataBlock2Algorand = blockqueue.Init()
	blockqueue.TableBlock2Algorand = blockqueue.Init()
	blockqueue.Algorand2DataBlock = blockqueue.Init()
	blockqueue.Algorand2TableBlock = blockqueue.Init()
	s := new(server.Server)
	serverExec.RPCs = s
	client.Cserver = s
	s.Init()

	// 判断集群文件是否存在，如果不存在则直接执行cmd程序
	_, err := os.Stat("./ClusterInfo")
	if os.IsNotExist(err) {
		go s.Command()
		go client.StartClient()
		serverExec.ServerStart()
	} else {
		// 启动Raft, 读取集群文件
		cluster, err := Cluster.LoadClusterFile("./ClusterInfo")
		if err != nil {
			log.Panic(err)
		}

		Cluster.LocalNode = cluster
		for _, node := range cluster.Node {
			fmt.Println(node.IP, node.Port)
		}

		go Cluster.Server()
		// 集群中节点的更新
		go cluster.UpdateClusterFile()
		// 新建algorand共识
		alg := algorand.NewAlgorand(util.LocalIP + strconv.Itoa(util.LocalPort))
		//go Raft.Start(&s.TxPool)
		alg.Start()

		go s.Command()
		go client.StartClient()
		serverExec.ServerStart()
	}

}
