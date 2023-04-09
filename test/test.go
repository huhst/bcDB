package main

import (
	"alg_bcDB/serverExec/service"
	"context"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"time"
)

const (
	defaultName = "world"
)

var (
	addr = flag.String("addr", "192.168.1.101:8888", "the address to connect to")
	//name = flag.String("name", defaultName, "Name to greet")
)

func main() {
	flag.Parse()

	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	//defer conn.Close()
	c := service.NewServerClient(conn)
	//ctx, _ := context.WithTimeout(context.Background(), time.Second*5)

	username, password := "wxz", "123"
	//key := "1M9"
	//value := "1M9"
	taableName := "ta1"
	//pli := "1M9ZdF3UwmTK7rZATPk5BA23m9V6K4Zn6u7"

	a1, err := c.Cmd(context.Background(), &service.CommandRequest{LoginAccount: username, LoginPassword: password})
	fmt.Println(a1.Login)
	if err != nil {
		log.Fatalf("could not sign in: %v", err)
	}

	uid := username + "-QAQ-" + password

	//_, err = c.Cmd(context.Background(), &service.CommandRequest{Uid: uid, TabelName: taableName, PermissionTable: pli})
	//fmt.Println(a1.Login)
	//if err != nil {
	//	log.Fatalf("could not sign in: %v", err)
	//}

	count := 0

	start1 := time.Now() // 获取当前时间
	for i := 0; i < 4000; i++ {
		key := "key" + string(i)
		//value := "value" + string(i)
		//_, err := c.Cmd(context.Background(), &service.CommandRequest{Uid: uid, TabelName: taableName, Key: key, Value: value})
		_, err := c.Cmd(context.Background(), &service.CommandRequest{Uid: uid, Key: key, TabelName: taableName})
		if err != nil {
			fmt.Println(err)
		}
		count++

		//fmt.Printf("%d\n", count)
	}
	//fmt.Printf("%d  ", count)

	elapsed := time.Since(start1)

	////fmt.Println("在表相关链中查找")
	//fmt.Println("在lru历史队列中查找")
	////fmt.Println("在lru缓存队列中查找")
	fmt.Printf("%d 个读取响应完成耗时：%v\n", count, elapsed)
	fmt.Println("每秒完成响应", float64(count)/(elapsed.Seconds()), "/s")
}
