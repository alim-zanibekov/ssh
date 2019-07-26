package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

func main() {
	forwardAddr := os.Getenv("SSH_PORT")
	serverAddr := os.Getenv("SERVER_SSH")
	if len(serverAddr) == 0 {
		serverAddr = fmt.Sprintf(":%d", 9999)
	}
	if len(forwardAddr) == 0 {
		forwardAddr = ":22"
	}
	args := os.Args[1:]
	token := "none:token"

	if len(args) > 1 {
		token = args[0] + ":" + args[1]
	}

	var (
		client net.Conn
		server net.Conn
	)

	client, server = nil, nil

	server, err := net.Dial("tcp", serverAddr)
	if err != nil {
		log.Fatalf("Failed to setup listener: %v", err)
	}
	log.Printf("Connected to server %v\n", server)

	err = binary.Write(server, binary.LittleEndian, int32(len(token)))
	if err != nil {
		fmt.Println("Token send error:", err)
	}
	server.Write([]byte(token))
	client, err = net.Dial("tcp", forwardAddr)
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	log.Printf("Connected to localhost %v\n", client)

	err = server.(*net.TCPConn).SetKeepAlive(true)
	if err != nil {
		fmt.Println("SetKeepAlive error :", err)
		server.Close()
		return
	}

	err = server.(*net.TCPConn).SetKeepAlivePeriod(5 * time.Second)
	if err != nil {
		fmt.Println("SetKeepAlivePeriod error :", err)
		server.Close()
		return
	}

	var exit = make(chan int)
	go func() {
		defer server.Close()
		defer client.Close()
		io.Copy(server, client)
		exit <- 1
	}()
	go func() {
		defer server.Close()
		defer client.Close()
		io.Copy(client, server)
		exit <- 1

	}()
	<-exit
}
