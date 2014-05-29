package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/vmihailenco/msgpack"
)

type MessageType int

const (
	ControlMessage MessageType = iota
	VoiceMessage
	TextMessage
)

type packet struct {
	bytes         []byte
	returnAddress *net.UDPAddr
}

type Message struct {
	Type        MessageType
	Destination string // roomID
	Message     []byte
}

func (c *client) connectToNetwork(address string) {
	addr, _ := net.ResolveUDPAddr("udp", address)
	log.Printf("ADDRESS IS: %#v\n", addr)
	token := c.Network.PingIP(addr)

	c.ui <- "...Connecting..."
	// check if successful - after 2 seconds?
	time.Sleep(500 * time.Millisecond)

	_, ok := c.Network.AwaitingResponse[token]

	if ok {
		c.ui <- "...Connection Failed"
		return // connection failed. TODO: REPORT BACK
	}

	c.ui <- fmt.Sprintf("...Connection OK, sending FindNodes now. Addr is: %s", addr)
	remote, ok := c.Node.AddressToNode[addr.String()]

	go c.Network.FindNode(remote, c.Node.ID)
	return
}

func (c *client) initNetwork() {
	address := fmt.Sprintf(":%d", c.port)
	listener, err := net.ListenPacket("udp4", address)

	if err != nil {
		// do something
		c.ui <- "...Unable to start UDP listener. Panicking now."
		panic(err) // temp. TODO: replace with actual error handling.
	}

	var socket *net.UDPConn
	if listener != nil {
		socket = listener.(*net.UDPConn)
	}

	c.connection = socket
}

func (c *client) readFromSocket() {
	for {
		var b []byte = make([]byte, 1024)
		n, addr, err := c.connection.ReadFromUDP(b)

		b = b[0:n]
		log.Printf("READ %d: %#v\n", len(b), b)

		if err != nil {
			panic("HELP!!") // do actual proper error handling kthxbai
		}

		if n > 0 {
			pack := packet{b, addr}
			select {
			case c.packets <- pack:
				continue
			case <-c.kill:
				break
			}
		}

		select {
		case <-c.kill:
			break
		default:
			continue
		}
	}
}

func (c *client) processPackets() {
	for pack := range c.packets {
		var msg Message
		err := msgpack.Unmarshal(pack.bytes, &msg)
		if err != nil {
			// do something
			log.Printf("FAILED TO UNMARSHAL")
			continue
		}

		c.messages <- msg
	}
}

func (c *client) processMessages() {
	for msg := range c.messages {
		if msg.Type == TextMessage {
			room, ok := c.chatroomsID[msg.Destination]
			if !ok {
				c.ui <- "...Unable to find chatroom"
				continue
			}
			friendlyName := room.Name
			log.Printf("Received TXT : %s\n", msg.Message)
			c.ui <- fmt.Sprintf("%s\n%s\n", friendlyName, string(msg.Message))
		}
	}
}

func sendMsg(conn *net.UDPConn, returnAddress *net.UDPAddr, msg Message) {
	b, err := msgpack.Marshal(msg)
	if err != nil {
		// do something!
	}
	log.Printf("Conn: %#v\nReturnAddr: %#v\n", conn, returnAddress)
	bytesWritten, networkErr := conn.Write(b)

	if networkErr != nil {
		log.Fatalf("FAILED TO WRITE %d BYTES. Reason: %v", bytesWritten, networkErr)
		return
	}
}

func (c *client) Send(id string, message string) {
	room, ok := c.chatroomsID[id]
	if !ok {
		c.ui <- fmt.Sprintf("...No such chatroom: %s", id)
	}

	msg := Message{
		Type:        TextMessage,
		Destination: room.ID,
		Message:     []byte(message),
	}

	rA := c.connection.LocalAddr()

	for _, v := range room.participants {
		conn, err := net.DialUDP("udp", nil, v)
		if err != nil {
			c.ui <- fmt.Sprintf("...Unable to create connection to %s", v)
		}
		returnAddr, ok := rA.(*net.UDPAddr)
		if !ok {
			log.Printf("Return Addr is %T\n", rA)
			c.ui <- "...Return Address is not a UDP Address"
		}
		go sendMsg(conn, returnAddr, msg)
	}
}
