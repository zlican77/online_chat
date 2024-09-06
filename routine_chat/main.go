package main

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Client struct {
	C    chan string //客户监听管道，接收消息
	Name string      //用户名
	Addr string      //用户地址
}

var onlineMap = make(map[string]Client) //在线用户
var messageChan = make(chan string)
var isQuit = make(chan bool)
var hasData = make(chan bool)

func makeMessage(cli Client, msg string) string {
	return "[" + cli.Name + "]" + msg
}

func Manager() { //处理分发消息chan给每一个用户，广播
	for {
		msg := <-messageChan
		for _, cli := range onlineMap {
			cli.C <- msg
		}
	}
}

func writeChanMessage(cli Client, conn net.Conn) {
	for msg := range cli.C {
		conn.Write([]byte(msg + "\n"))
	}
}

func writeClientMessage(cli Client, conn net.Conn) { //服务器端拿到用户的数据，广播
	for {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println(err)
			return
		}
		msg := string(buf[:n-1])
		if len(msg) == 0 {
			isQuit <- true
		}
		if n == 4 && msg == "who" { //查询在线用户
			conn.Write([]byte("user list:\n"))
			for _, cli := range onlineMap {
				conn.Write([]byte(cli.Name + "|" + cli.Addr + "\n"))
			}
			continue
		} else if len(msg) >= 8 && msg[:6] == "rename" {
			name := strings.Split(msg, "|")[1]
			cli.Name = name
			onlineMap[cli.Addr] = cli
			conn.Write([]byte("rename " + cli.Name + " success\n"))
			continue
		} else {
			messageChan <- makeMessage(cli, msg)
		}
		hasData <- true
	}
}

// 处理用户链接
func handleConn(conn net.Conn) {
	defer conn.Close()

	cliAddr := conn.RemoteAddr().String()
	var c = make(chan string)
	cli := Client{
		C:    c,
		Name: string(cliAddr),
		Addr: string(cliAddr),
	}
	go writeChanMessage(cli, conn) //监听打印自身管道消息

	onlineMap[cli.Name] = cli //将存在用户写入服务器
	messageChan <- makeMessage(cli, "用户登入成功")
	cli.C <- makeMessage(cli, "i am here")

	go writeClientMessage(cli, conn) //广播用户的输入消息

	for {
		select {
		case <-isQuit:
			delete(onlineMap, cliAddr)
			messageChan <- makeMessage(cli, "退出聊天")
			return
		case <-hasData:

		case <-time.After(60 * time.Second):
			delete(onlineMap, cliAddr)
			messageChan <- makeMessage(cli, "超时退出")
			return
		}
	}
}

func main() {
	//开启监听服务器
	listenner, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println(err)
		return
	}

	defer listenner.Close()

	go Manager()

	for { //监听客户端与用户建立连接
		conn, err := listenner.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		// 处理用户链接
		go handleConn(conn)
	}
}
