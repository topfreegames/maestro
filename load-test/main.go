package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"
)

var sentMessages int64
var receivedMessages int64
var wg sync.WaitGroup

func printDetails(duration float64) {
	log.Println("---------------------------------------")
	log.Printf("Total of sent messages: %d\n", sentMessages)
	log.Printf("Total of received messages: %d\n", receivedMessages)
	log.Printf("Percentage of received messages: %.2f%%\n", (float64(receivedMessages) / float64(sentMessages) * 100))
	log.Printf("Sent messages rate: %.2f/sec\n", float64(sentMessages)/duration)
	log.Printf("Received messages rate: %.2f/sec\n", float64(receivedMessages)/duration)
}

func echoHandler(conn *net.UDPConn, debug bool) {
	wg.Add(1)
	defer func() { wg.Done() }()
	buf := make([]byte, 1024)

	for {
		nr, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("Error receiving msg: ", err)
		} else {
			atomic.AddInt64(&receivedMessages, 1)
			if debug {
				log.Println("Received ", string(buf[:nr]), " from ", addr)
			}
		}

		nw, err := (*conn).WriteToUDP(buf[:nr], addr)
		if err != nil {
			log.Println("Error sending msg: ", err)
		} else if nr == nw {
			atomic.AddInt64(&sentMessages, 1)
			if debug {
				log.Println("Echoed ", string(buf[:nw]), " to ", addr)
			}
		} else {
			log.Println(fmt.Sprintf("received %d bytes but sent %d", nr, nw))
		}
	}
}

func server(numServers int, port int, debug bool) {
	start := time.Now()

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	log.Println("UDP Echo server listenning on", addr.String())

	for i := 0; i < numServers; i++ {
		go echoHandler(conn, debug)
	}

	// Print stats from time to time
	go func() {
		for {
			duration := time.Now().Sub(start)
			printDetails(duration.Seconds())
			time.Sleep(10 * time.Second)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Ctrl+C detected. Quitting...")
		duration := time.Now().Sub(start)
		printDetails(duration.Seconds())
		os.Exit(0)
	}()

	wg.Wait()
}

func clientSender(conn *net.UDPConn, mgsPerSec int, debug bool) {
	wg.Add(1)
	defer func() { wg.Done() }()

	msg := "t6tg2otaANXcnr6pX79RwWAeHBkpAvBqOe0AdZBHgRweiMJ5OSDOmgPYPssustvRiOL5hui5dBvNzGGCMef9PRMlnJoMnNQf9Tt8"
	for {
		buf := []byte(msg)
		_, err := conn.Write(buf)
		if err != nil {
			log.Println("Error sending msg: ", err)
		} else {
			atomic.AddInt64(&sentMessages, 1)
			if debug {
				log.Println("Echoed ", string(buf), " to ", conn.RemoteAddr())
			}
		}
		time.Sleep(time.Millisecond * time.Duration(1000/mgsPerSec))
	}
}

func clientListener(conn *net.UDPConn, debug bool) {
	wg.Add(1)
	defer func() { wg.Done() }()

	buf := make([]byte, 1024)

	for {
		nr, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("Error receiving msg: ", err)
		} else {
			atomic.AddInt64(&receivedMessages, 1)
			if debug {
				log.Println("Received ", string(buf[:nr]), " from ", addr)
			}
		}
	}
}

func client(numClients int, mgsPerSec int, ipsFilePath string, debug bool) {
	start := time.Now()

	ipsFile, err := os.Open(ipsFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { ipsFile.Close() }()
	scanner := bufio.NewScanner(ipsFile)

	for scanner.Scan() {
		serverAddrStr := scanner.Text()
		serverAddr, err := net.ResolveUDPAddr("udp", serverAddrStr)
		if err != nil {
			log.Fatal(err)
		}

		for i := 0; i < numClients; i++ {
			localAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
			if err != nil {
				log.Fatal(err)
			}

			conn, err := net.DialUDP("udp", localAddr, serverAddr)
			if err != nil {
				log.Fatal(err)
			}
			defer conn.Close()
			log.Println("UDP Client sending to", serverAddr.String())

			go clientListener(conn, debug)
			go clientSender(conn, mgsPerSec, debug)
		}
	}

	// Print stats from time to time
	go func() {
		for {
			duration := time.Now().Sub(start)
			printDetails(duration.Seconds())
			time.Sleep(10 * time.Second)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Ctrl+C detected. Quitting...")
		duration := time.Now().Sub(start)
		printDetails(duration.Seconds())
		os.Exit(0)
	}()

	wg.Wait()
}

func main() {
	debugPtr := flag.Bool("debug", false, "verbose logging")
	portPtr := flag.Int("port", 5050, "server port")
	numPtr := flag.Int("proc", 1, "number of concurrent processes")
	msgsPtr := flag.Int("msgs", 10, "number of messages per second")

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("You must specify a mode (server or client)")
		os.Exit(1)
	}
	switch args[0] {
	case "server":
		server(*numPtr, *portPtr, *debugPtr)
	case "client":
		if len(args) < 2 {
			fmt.Println("You must specify a file with ips and ports to send messages to")
			os.Exit(1)
		}
		client(*numPtr, *msgsPtr, args[1], *debugPtr)
	default:
		fmt.Printf("%s mode is not supported\n", args[0])
		os.Exit(1)
	}
}
