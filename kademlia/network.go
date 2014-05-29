package kademlia

import (
	"fmt"
	"log"
	"net"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/vmihailenco/msgpack"
)

type packet struct {
	bytes         []byte
	returnAddress *net.UDPAddr
}

// this is exported to allow for new query types although now only the 4 basic kademlia queries will be allowed
type Message struct {
	MessageType string
	SourceID    NodeID
	Token       string
	Message     interface{}
}

func NewMessage() (Message, string) {
	token := uuid.New()
	return Message{Token: token}, token
}

func (msg *Message) InsertMessage(message interface{}) {
	marshalled, err := msgpack.Marshal(message)
	if err != nil {
		// do something about it
	}
	msg.Message = marshalled
}

func SendMsg(conn *net.UDPConn, returnAddress *net.UDPAddr, msg Message) {
	b, err := msgpack.Marshal(msg)
	if err != nil {
		// do something!
	}
	bytesWritten, networkErr := conn.WriteToUDP(b, returnAddress)

	if networkErr != nil {
		log.Fatalf("FAILED TO WRITE %d BYTES. Reason: %v", bytesWritten, networkErr)
	}
}

func (dht *Kademlia) initNetwork() {
	address := fmt.Sprintf(":%d", dht.Node.Port)

	listener, err := net.ListenPacket("udp4", address)

	if err != nil {
		// do something
		panic(err) // temp. TODO: replace with actual error handling.
	}

	var socket *net.UDPConn
	if listener != nil {
		socket = listener.(*net.UDPConn)
	}

	dht.Connection = socket
}

func (dht *Kademlia) readFromSocket() {
	for {
		var b []byte = make([]byte, 1024)
		n, addr, err := dht.Connection.ReadFromUDP(b)

		b = b[0:n]

		if err != nil {
			panic("HELP!!") // do actual proper error handling kthxbai
		}

		if n > 0 {
			pack := packet{b, addr}
			select {
			case dht.packets <- pack:
				continue
			case <-dht.kill:
				break
			}
		}

		select {
		case <-dht.kill:
			break
		default:
			continue
		}
	}
}

func (dht *Kademlia) processPackets() {
	// packets are all Messages as a byte array. processPacket() basically verifies this, and errors out if weird shit packets comes in
	for pack := range dht.packets {
		var msg Message
		err := msgpack.Unmarshal(pack.bytes, &msg)
		if err != nil {
			// do something
			log.Printf("Failed to unmarshal message in packet.")
			continue
		}

		//check and see if node exists
		remote := dht.Node.GetOrCreateNode(msg.SourceID, pack.returnAddress.String())
		dht.pendingEnvelopes[msg.Token] = remote

		dht.requests <- msg
	}
}

// yay for replicating Go's basic RPC functions
func (dht *Kademlia) handleMessages() {
	for msg := range dht.requests {
		_, ok := dht.pendingQueries[msg.Token]
		if ok { // todo: check the time since
			continue // request is being worked on.
		}

		dht.pendingQueries[msg.Token] = time.Now()

		f, ok := dht.ResponseHandler[msg.MessageType]
		if !ok {
			// BAD SHIT HAPPENS HERE
			log.Printf("DISCARDED (No Response Handler): %#v \n", msg.MessageType)
			delete(dht.pendingQueries, msg.Token)
			continue
		}

		remote, ok := dht.pendingEnvelopes[msg.Token]
		if !ok {
			// BAD SHIT HAPPENS HERE
		}
		go f(remote, msg.Token, msg.SourceID, msg.Message)
		delete(dht.pendingQueries, msg.Token) // once that is done, delete pending stuff
		delete(dht.pendingEnvelopes, msg.Token)
	}
}
