package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"time"
)

func main() {
	debugStateStr := os.Getenv("DEBUG")
	agentAddr := os.Getenv("SERVER_AGENT")
	user := os.Getenv("SSH_USER")
	pass := os.Getenv("SSH_PASSWORD")

	if len(agentAddr) == 0 {
		agentAddr = fmt.Sprintf(":%d", 8888)
	}

	debugging := false
	if debugStateStr == "1" || debugStateStr == "true" {
		debugging = true
	}

	args := os.Args[1:]
	id := "none"
	if len(args) > 0 {
		id = args[0]
	}

	if debugging {
		log.Println("Server addr: ", agentAddr)
		log.Println("Token: ", id)
		log.Println("SSH user: ", user)
		log.Println("SSH pass: ", pass)
	}

	for {
		var (
			server net.Conn
		)

		message := id + ":" + user + ":" + pass
		server, err := net.Dial("tcp", agentAddr)
		if err != nil {
			log.Printf("Failed to setup listener: %v", err)
			time.Sleep(2000000000)
			continue
		}
		log.Printf("Connected to server %v\n", server)

		err = binary.Write(server, binary.LittleEndian, int32(len(message)))
		if err != nil {
			fmt.Println("Message size send error :", err)
		}
		_, err = server.Write([]byte(message))
		if err != nil {
			fmt.Println("Message send error:", err)
			server.Close()
			continue
		}

		err = server.(*net.TCPConn).SetKeepAlive(true)
		if err != nil {
			fmt.Println("SetKeepAlive error :", err)
			server.Close()
			return
		}

		err = server.(*net.TCPConn).SetKeepAlivePeriod(10 * time.Second)
		if err != nil {
			fmt.Println("SetKeepAlivePeriod error :", err)
			server.Close()
			return
		}

		for {
			var buf []byte
			buf = make([]byte, 256)
			_, err := server.Read(buf)
			if err != nil {
				fmt.Println("Read from server error:", err)
				server.Close()
				break
			}

			token := randSeq(30)

			go func() {
				cmd := exec.Command("/bin/console-ssh-connector", id, token)
				log.Printf("Running node...")
				err := cmd.Run()
				log.Printf("Node finished with error: %v", err)
			}()

			err = binary.Write(server, binary.LittleEndian, int32(len(token)))
			if err != nil {
				fmt.Println("Token size send error :", err)
			}
			_, err = server.Write([]byte(token))
			if err != nil {
				fmt.Println("Token send error :", err)
			}
		}
	}
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
