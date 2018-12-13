// BACK END SERVER FOR
// Quotes, The Best Quotes

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

var backendConns []*net.TCPConn
var backendConnMutex sync.RWMutex

var term int

// Global Slice that Holds the strings of quotes
var quotes = []string{}

// Read/Write Mutex
var mutex sync.RWMutex

var currentLeader *net.TCPConn

// Message comment
type Message struct {
	termNum int
}

var voteSuccessful chan bool
var resetTimeout chan bool

// Adds a new user inputted quote into the global slice
func createQuote(a string) {
	mutex.Lock()
	quotes = append(quotes, a)
	mutex.Unlock()
}

// Deletes the selected quote
// *** Could be better implemented if quotes were stored with an identifier. Problems
// may arise if two quotes are exactly the same ***
func deleteQuote(id string) {
	index := 0
	mutex.Lock()
	for i, v := range quotes {
		if v == id {
			index = i
			break
		}
	}
	quotes = quotes[:index+copy(quotes[index:], quotes[index+1:])]
	mutex.Unlock()
}

// Edits/updates a quote with changes the user inputs
func updateQuote(id string, newQuote string) {
	index := 0
	mutex.Lock()
	for i, v := range quotes {
		if v == id {
			index = i
			break
		}
	}
	quotes[index] = newQuote
	mutex.Unlock()
}

// Handles each TCP connection from frontends
func connHandler(conn net.Conn, backendAddrs []*net.TCPAddr) {
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	allBackendsConnected := false

	// Reads actions from the front end
	for {
		var action string
		decoder.Decode(&action)
		switch action {
		case "PING":
			encoder.Encode("ACK")
		case "INITSYNC":
			if allBackendsConnected == false {
				allBackendsConnected = true
				initConnectToBackends(backendAddrs)
			}
		case "index":
			mutex.RLock()
			encoder.Encode(quotes)
			mutex.RUnlock()
		case "create":
			var newQuote string
			decoder.Decode(&newQuote)
			createQuote(newQuote)
		case "delete":
			var id string
			decoder.Decode(&id)
			deleteQuote(id)
		case "update":
			var id string
			var newQuote string
			decoder.Decode(&id)
			decoder.Decode(&newQuote)
			updateQuote(id, newQuote)
		}
	}
}

// goroutine that handles messages from each backend server
func backendConnHandler(conn *net.TCPConn, LeaderChan chan *net.TCPConn) {
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	//voted := false
	//var elecTerm int

	// map that uses the term number as the key with the value as the candidate voted
	termVoted := make(map[int]*net.TCPConn)

	for {
		var backendAction string
		decoder.Decode(&backendAction)
		switch backendAction {
		case "LEADER_HEARTBEAT":
			var heartbeatTerm int
			decoder.Decode(&heartbeatTerm)
			if heartbeatTerm >= term {
				LeaderChan <- conn
			} // else tell this node they have been usurped
		case "VOTE_REQUEST":
			var elecTerm int
			decoder.Decode(&elecTerm)
			//if elecTerm > term {
			_, ok := termVoted[elecTerm]
			if ok == false {
				termVoted[elecTerm] = conn
				encoder.Encode("VOTE_SUCCESS")
				encoder.Encode(Message{termNum: term})
			}
			//}
		case "VOTE_SUCCESS":
			resetTimeout <- true
			var messageTerm Message
			decoder.Decode(&messageTerm)
			if messageTerm.termNum > term {
				voteSuccessful <- true
			} else {
				voteSuccessful <- false
			}
		case "I_AM_LEADER":
			var x int
			decoder.Decode(&x)
			term = x
			LeaderChan <- conn
		}

	}
}

// Generates random timeout between 150 ms and 300 ms
func timeoutGenerator() time.Duration {
	return time.Duration(rand.Intn(151)+150) * time.Millisecond
}

// Synconization of backends using the Raft concensus algorithm
func raftConsensus(LeaderChan chan *net.TCPConn) {
	type State int
	const (
		Follower  State = 0
		Candidate State = 1
		Leader    State = 2
	)
	// log
	term = 0   // put in for loop
	votes := 0 // put in for loop

	backendConnMutex.RLock()
	quorum := len(backendConns) + 1
	backendConnMutex.RUnlock()
	currentState := Follower

	// Generates random timeout between 150 ms and 300 ms
	rand.Seed(time.Now().UnixNano())
	electionTimeout := timeoutGenerator()

	// If the current node is not leader
	// for loop?
	for {
		if currentState == Leader {
			backendConnMutex.RLock()
			for _, v := range backendConns {
				encoder := json.NewEncoder(v)
				encoder.Encode("LEADER_HEARTBEAT")
				encoder.Encode(term)
			}
			backendConnMutex.RUnlock()

		} else {

			select {
			case currentLeader = <-LeaderChan:
				// There is leader
			case <-resetTimeout:
				electionTimeout = timeoutGenerator()
			case <-time.After(electionTimeout):
				// No leader detected. Start election
				currentState = Candidate
				term++
				votes = 1
				electionTimeout = timeoutGenerator()
				backendConnMutex.RLock()
				for _, v := range backendConns {
					// Sends a vote request along with the term number to all replica nodes
					encoder := json.NewEncoder(v)
					encoder.Encode("VOTE_REQUEST")
					encoder.Encode(term)
				}
				backendConnMutex.RUnlock()

				// Waiting for votes to come in
				for {
					select {
					case validVote := <-voteSuccessful:
						if validVote == true {
							votes++
							if votes >= quorum {
								currentState = Leader
								backendConnMutex.RLock()
								for _, v := range backendConns {
									encoder := json.NewEncoder(v)
									encoder.Encode("I_AM_LEADER")
									encoder.Encode(term)
								}
								backendConnMutex.RUnlock()
								break
							}
						} else {
							currentState = Follower
							break
						}
					case <-time.After(electionTimeout):
						currentState = Follower
						break
					}
				}

			}
		}
	}

}

// Initial connections to start synchronization between all backends
func initConnectToBackends(backendAddrs []*net.TCPAddr) {
	// channel for detecting heartbeats from leader
	LeaderChan := make(chan *net.TCPConn)

	for _, v := range backendAddrs {
		conn, err := net.DialTCP("tcp", nil, v)
		if err != nil {
			fmt.Println("A backend server could not be connected to.")
			//return
		}
		go backendConnHandler(conn, LeaderChan)
		backendConnMutex.Lock()
		backendConns = append(backendConns, conn)
		backendConnMutex.Unlock()
	}
	go raftConsensus(LeaderChan)
	return
}

func main() {

	// Preliminary hardcoded data
	quotes = append(quotes, "If a book about failures doesn’t sell, is it a success? - Jerry Seinfeld")
	quotes = append(quotes, "I like long walks, especially when they are taken by people who annoy me. - Fred Allen")
	quotes = append(quotes, "The most powerful force in the universe is compound interest. - Albert Einstein")
	quotes = append(quotes, "I love mankind. It's people I can't stand. - Charles M. Schulz")
	quotes = append(quotes, "Laziness is nothing more than the habit of resting before you get tired. - Jules Renard")
	quotes = append(quotes, "Go to Heaven for the climate, Hell for the company. - Mark Twain")
	quotes = append(quotes, "I can resist everything except temptation. - Oscar Wilde")
	quotes = append(quotes, "A committee is a group that keeps minutes and loses hours. - Milton Berle")

	//go quoteCheck()

	// front end listen flag
	var portnum int
	flag.IntVar(&portnum, "listen", 8090, "front end listening port")
	// backend tcp flag
	var backendtcp string
	flag.StringVar(&backendtcp, "backend", "", "backend TCP port")
	flag.Parse()

	// converts front end flag to string
	p := strconv.Itoa(portnum)
	port := ":" + p

	// Array that holds the backend TCP ports
	backendNodes := strings.Split(backendtcp, ",")

	// Slice that holds the resolved backend TCP addresses
	var backendAddrs []*net.TCPAddr
	for _, v := range backendNodes {
		temp, _ := net.ResolveTCPAddr("tcp", v)
		backendAddrs = append(backendAddrs, temp)
	}

	// Listens on this port
	ln, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Println("Error listening on TCP port")
	}

	for {
		// Establishes a TCP connection to the frontend
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Error accepting connection")
		}
		fmt.Printf("Connection accepted on port %s.\n", port)
		go connHandler(conn, backendAddrs)
	}
	//defer conn.Close()
}
