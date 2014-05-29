package main

import (
	"github.com/chewxy/nanjingtaxi/kademlia"
	"github.com/kr/pretty"

	"fmt"
	"os"
	"strconv"
	"log"
	"bufio"
	"net"
	"strings"

	"crypto/rsa"
)

type client struct{
	Node *kademlia.Node
	Network *kademlia.Kademlia

	port int
	connection *net.UDPConn

	packets chan packet
	messages chan Message
	kill chan bool

	ui chan string

	privateKey *rsa.PrivateKey
	chatroomsID map[string]*chatroom
	chatroomsName map[string]*chatroom

}

func newClient() *client {
	return &client{
		Node: kademlia.NewNode(),

		packets: make(chan packet),
		messages: make(chan Message),
		kill: make(chan bool),

		ui: make(chan string),

		chatroomsID: make(map[string]*chatroom),
		chatroomsName: make(map[string]*chatroom),
	}
}


func (c *client) inputloop() {
	reader := bufio.NewReader(os.Stdin)
	for {
		c.ui <- " "
		// var input string
		// fmt.Scanf("%s", &input)

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "cx":
			c.ui <- "Address:"
			argAddr, _ := reader.ReadString('\n')
			argAddr = strings.TrimSpace(argAddr)
			
			c.connectToNetwork(argAddr)
		case "nodes":
			c.ui <- "Nodes"
			for i, bucket := range c.Node.RoutingTable {
				c.ui <- fmt.Sprintf("\tBucket Number: %d", i)
				for elem := bucket.Front(); elem != nil; elem = elem.Next() {
					r, _ := elem.Value.(*kademlia.RemoteNode)
					c.ui <- fmt.Sprintf("\t\tID: %v\n\t\tAddr: %s\n\t\t===", r.ID, r.Address)
				}
			}

		case "join":
			c.ui <- "Room ID:"
			argID, _ := reader.ReadString('\n')
			argID = strings.TrimSpace(argID)
			c.RequestRoom(argID)

		case "new":
			c.ui <- "Room Name:"

			argName, _ := reader.ReadString('\n')
			argName = strings.TrimSpace(argName)

			c.ui <- "...Generating Chatroom..."
			chatRoom := createChatroom()
			chatRoom.Name = argName
			c.chatroomsID[chatRoom.ID] = chatRoom
			c.chatroomsName[argName] = chatRoom
			
			// add own address to participants
			c.ui <- "...Updating Chatroom..."
			lA := c.connection.LocalAddr()
			localAddr, _ := lA.(*net.UDPAddr)
			chatRoom.participants[string(c.Node.ID)] = localAddr

			chatRoom.ExportKeys()

			// store room ID in the kademlia network so that people can find the room
			c.Network.LocalStore(chatRoom.ID, c.Node.ID)

			c.ui <- fmt.Sprintf("...Chatroom Created. \nID: %s. \nUser Friendly Name: %s\nThe keys to this room are: chatrooms/%s.pem", chatRoom.ID, chatRoom.Name, chatRoom.ID)

		case "ls":
			c.ui <- "Chatrooms - "
			for _, cr := range c.chatroomsID {
				c.ui <- fmt.Sprintf("\t%s (%s)", cr.Name, cr.ID)
				c.ui <- "\tParticipants - "
				for k, v := range cr.participants {
					c.ui <- fmt.Sprintf("\t\t%s - %s", k, v.String())
				}
			}

		case "self":
			c.ui <- fmt.Sprintf("I am:\n\t%#v", c.Node.ID)
			c.ui <- fmt.Sprintf("\tConnection: %s", c.connection.LocalAddr())
			c.ui <- fmt.Sprintf("\tRequests Waiting: \n\t\t%# v", pretty.Formatter(c.Network.AwaitingResponse))

		case "send":
			c.ui <- "Room ID:"
			argID, _ := reader.ReadString('\n')
			argID = strings.TrimSpace(argID)

			c.ui <- "Message:"
			msg, _ := reader.ReadString('\n')
			msg = strings.TrimSpace(msg)
			c.Send(argID, msg)

		case "invite":
			c.ui <- "Room ID:"
			argID, _ := reader.ReadString('\n')
			argID = strings.TrimSpace(argID)

			chatRoom, ok := c.chatroomsID[argID]
			if !ok {
				c.ui <- fmt.Sprintf("Chatroom %s not found\n", argID)
				continue
			}
			c.ui <- "...Generating Invite..."
			chatRoom.GenerateInvite()
			c.ui <- "...Done Generating Invite."
		}
	
	}
}

func main() {
	// check if all the directories exist. If not, create them
	_, err := os.Stat("chatrooms/")
	if os.IsNotExist(err) {
		os.Mkdir("chatrooms/", os.ModeDir|os.ModePerm)
	}

	_, err = os.Stat("keys/")
	if os.IsNotExist(err) {
		os.Mkdir("keys/", os.ModeDir|os.ModePerm)
	}

	_, err = os.Stat("invites/")
	if os.IsNotExist(err) {
		os.Mkdir("invites/", os.ModeDir|os.ModePerm)
	}

	log.Println(os.Args)
	c := newClient()
	c.Node.Port, _ = strconv.Atoi(os.Args[1])
	c.port, _ = strconv.Atoi(os.Args[2])

	c.initNetwork()

	c.Network = kademlia.NewKademlia()
	c.Network.Node = c.Node

	// register new handlers with Network
	c.Network.ResponseHandler["REQUEST_ROOM"] = c.issueChallenge
	c.Network.ResponseHandler["CHALLENGE"] = c.challengeResponse
	c.Network.ResponseHandler["CHALLENGE_RESPONSE"] = c.verifyChallengeResponse
	c.Network.ResponseHandler["GROUP_PRIVATE_KEY"] = c.receiveGroupPrivateKey


	go c.Network.Run()

	go c.readFromSocket()

	go c.processPackets()

	go c.processMessages()

	go c.uiloop()

	c.inputloop()
}