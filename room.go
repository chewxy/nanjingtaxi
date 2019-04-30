package main

import (
	"github.com/agl/pond/bbssig"
	"github.com/chewxy/nanjingtaxi/kademlia"
	"github.com/google/uuid"
	"github.com/vmihailenco/msgpack"

	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	// "crypto/rsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	// "crypto/x509"
	"encoding/pem"
)

type chatroom struct {
	ID   string
	Name string

	participants map[string]*net.UDPAddr
	publicKeys   map[string]*rsa.PublicKey

	groupPrivateKey *bbssig.PrivateKey
	groupPublicKey  *bbssig.Group

	memberPrivateKey *bbssig.MemberKey

	trustedPeers []*kademlia.RemoteNode

	valid bool
}

// Creates a chatroom, autogenerating all the keys for the chatroom
func createChatroom() *chatroom {
	id := uuid.New().String()

	groupPriv, err := bbssig.GenerateGroup(rand.Reader)

	if err != nil {
		log.Fatalf("Error while generating group key: %s", err)
		panic("Error!")
	}

	memberPriv, err := groupPriv.NewMember(rand.Reader)

	if err != nil {
		log.Fatalf("Create Chatroom: Error while generating member key: %s", err)
		panic("Error!")
	}

	chatRoom := newChatroom(id, groupPriv, groupPriv.Group, memberPriv)
	chatRoom.valid = true

	return chatRoom
}

func newChatroom(id string, groupKey *bbssig.PrivateKey, groupPublicKey *bbssig.Group, memberKey *bbssig.MemberKey) *chatroom {
	return &chatroom{
		ID:           id,
		participants: make(map[string]*net.UDPAddr),

		groupPrivateKey:  groupKey,
		groupPublicKey:   groupPublicKey,
		memberPrivateKey: memberKey,

		trustedPeers: make([]*kademlia.RemoteNode, 0),

		valid: false,
	}
}

// exports the keys of a chatroom.
func (room *chatroom) ExportKeys() {
	// public key of the room
	publicFilename := fmt.Sprintf("chatrooms/%s_public.pem", room.ID)
	publicPemFile, err := os.Create(publicFilename)

	if err != nil {
		log.Fatalf("Failed to open %s_public.pem for writing: %s", room.ID, err)
		return
	}
	pem.Encode(publicPemFile, &pem.Block{Type: "GROUP PUBLIC KEY", Bytes: room.groupPrivateKey.Group.Marshal()})
	publicPemFile.Close()

	// private key of the room - this is the key that allows creation of new members
	privateFilename := fmt.Sprintf("chatrooms/%s_private.pem", room.ID)
	privatePemFile, err := os.Create(privateFilename)

	if err != nil {
		log.Fatalf("Failed to open %s_private.pem for writing: %s", room.ID, err)
		return
	}
	pem.Encode(privatePemFile, &pem.Block{Type: "GROUP PRIVATE KEY", Bytes: room.groupPrivateKey.Marshal()})
	privatePemFile.Close()

	// a member's private key
	memberFileName := fmt.Sprintf("keys/%s_member.pem", room.ID)
	memberPemFile, err := os.Create(memberFileName)

	if err != nil {
		log.Fatalf("Failed to open %s_member.pem for writing: %s", room.ID, err)
		return
	}
	pem.Encode(memberPemFile, &pem.Block{Type: "MEMBER PRIVATE KEY", Bytes: room.memberPrivateKey.Marshal()})
	memberPemFile.Close()
}

// RequestRoom sends a message via the Kademlia network, looking for nodes with the chatroom ID
func (c *client) RequestRoom(ID string) {
	// find a node with the chatroom id first
	tmpRemote := c.Network.Node.GetNearestNode()
	log.Printf("Nearest Node is: %#v\n", tmpRemote.ID)

	if tmpRemote == nil {
		c.ui <- "...No remote node found" // typically because well, the client is not connected to the kademlia network.
		return
	}

	// issue FIND_VALUE
	token := c.Network.FindValue(tmpRemote, ID)
	ch, ok := c.Network.ResultChan[token]
	defer delete(c.Network.ResultChan, token) // clean up after oneself

	if !ok {
		c.ui <- "...No result channel created for room request"
		return
	}

	var id kademlia.NodeID
	tmp := <-ch
	asBytes, ok := tmp.([]byte)

	if !ok {
		panic("SHIT SHIT")
	}

	err := msgpack.Unmarshal(asBytes, &id)
	if err != nil {
		c.ui <- fmt.Sprintf("...Unable to unmarshal. Error was: %s", err)
		return
	}

	// found the ID? get the remoteNode so we can send it a message asking for a challenge
	remote := c.Network.Node.GetNode(id)

	// get the relevant room settings - member key
	memberFileName := fmt.Sprintf("keys/%s_member.pem", ID)
	memberPemData, err := ioutil.ReadFile(memberFileName)
	if err != nil {
		//shit
		c.ui <- fmt.Sprintf("...No pem file available for room request. It should be in keys/%s_member.pem", ID)
		return
	}

	block, _ := pem.Decode(memberPemData)
	if block == nil {
		// shit
		c.ui <- fmt.Sprintf("...block is not a pem encoded file. Check keys/%s_member.pem", ID)
	}
	if block.Type != "MEMBER PRIVATE KEY" {
		c.ui <- "...Incorrect pem type. Expected MEMBER PRIVATE KEY"
		return
	}

	// get the relevant room settings - public key
	publicFileName := fmt.Sprintf("chatrooms/%s_public.pem", ID)
	publicPemData, err := ioutil.ReadFile(publicFileName)
	if err != nil {
		// shit
		c.ui <- fmt.Sprintf("...No room publick pem file available. It should be in chatrooms/%s_public.pem", ID)
		return
	}

	publicBlock, _ := pem.Decode(publicPemData)
	if publicBlock == nil {
		//shit
		c.ui <- fmt.Sprintf("...Public block is not a pem encoded file. Check chatrooms/%s_public.pem", ID)
		return
	}
	if publicBlock.Type != "GROUP PUBLIC KEY" {
		c.ui <- "...Incorrect pem type. Expected GROUP PUBLIC KEY"
		return
	}

	// create a dummy chatroom. The dummy chatroom is required because to unmarshal the keys, a key is needed to begin with
	chatRoom := createChatroom()
	c.chatroomsID[ID] = chatRoom
	chatRoom.ID = ID

	group, success := chatRoom.groupPublicKey.Unmarshal(publicBlock.Bytes)
	if !success {
		c.ui <- "...Unable to unmarshal group public key"
		return
	}

	memberPriv, success := chatRoom.memberPrivateKey.Unmarshal(group, block.Bytes)
	if !success {
		c.ui <- "...Unable to unmarshal member private key"
		return
	}

	// settings that are important to the challenge - these are dummy keys which are required to unmarshal
	chatRoom.groupPublicKey = group
	chatRoom.memberPrivateKey = memberPriv

	// send message
	message, token := kademlia.NewMessage()
	message.MessageType = "REQUEST_ROOM"
	message.SourceID = c.Network.Node.ID
	message.Message = ID

	kademlia.SendMsg(c.Network.Connection, remote.Address, message)

	// register things with the token so the reply knows wtf is going on
	c.Network.AwaitingResponse[token] = time.Now()
	c.Network.ExtraInfo[token] = ID
}

// issueChallenge is a kademlia.ResponseFunc, hence the elaborate signature
// remote is the address on the envelope (i.e. the room requester)
// c is the challenge issuer
func (c *client) issueChallenge(remote *kademlia.RemoteNode, token string, source kademlia.NodeID, data interface{}) {
	roomID, _ := data.(string)
	// todo : what if data is not a string

	message, _ := kademlia.NewMessage()
	message.MessageType = "CHALLENGE"
	message.SourceID = c.Network.Node.ID
	message.Message = roomID
	message.Token = token

	kademlia.SendMsg(c.Network.Connection, remote.Address, message)

	c.Network.AwaitingResponse[token] = time.Now()
	c.Network.ExtraInfo[token] = roomID

}

type answerPacket struct {
	ChallengeAnswer string
	Port            int
}

func answerChallenge(message string, memberKey *bbssig.MemberKey) string {
	out, err := memberKey.Sign(rand.Reader, []byte(message), sha1.New())
	if err != nil {
		log.Printf("Challenge failed. Error was: %s\n", err)
	}

	return string(out)
}

// challengeResponse is a kademlia.ResponseFunc, hence the elaborate signature
// remote is the challenge issuer
// c is the room requester
func (c *client) challengeResponse(remote *kademlia.RemoteNode, token string, source kademlia.NodeID, data interface{}) {
	// no deleting from AwaitingResponse because the process is not complete yet

	roomID, ok := data.(string)
	if !ok {
		panic("data is not a string. No Room ID")
	}

	chatRoom, ok := c.chatroomsID[roomID]
	if !ok {
		log.Printf("Chatroom not found when responding to challenge. ID: %#v\n", c.chatroomsID)
		c.ui <- "...Chatroom not found when responding to challenge"
		return
	}

	answer := answerChallenge(roomID, chatRoom.memberPrivateKey)

	message, _ := kademlia.NewMessage()
	message.MessageType = "CHALLENGE_RESPONSE"
	message.SourceID = c.Network.Node.ID
	message.InsertMessage(answerPacket{answer, c.port})
	message.Token = token

	kademlia.SendMsg(c.Network.Connection, remote.Address, message)

	// update last response time
	c.Network.AwaitingResponse[token] = time.Now()
}

type validChallengeResponse struct {
	ChatroomID   string
	PrivateKey   []byte
	Name         string
	Port         int
	Participants map[string]*net.UDPAddr
}

// verifyChallengeResponse is a kademlia.ResponseFunc, hence the elaborate signature
// remote is the room requester
// c is the challenge issuer, and also the verifier
func (c *client) verifyChallengeResponse(remote *kademlia.RemoteNode, token string, source kademlia.NodeID, data interface{}) {
	defer delete(c.Network.AwaitingResponse, token)
	defer delete(c.Network.ExtraInfo, token)

	response, ok := data.(string)
	if !ok {
		// shit
	}

	var answer answerPacket
	err := msgpack.Unmarshal([]byte(response), &answer)

	if err != nil {
		// shit - TODO
	}

	chatRoomID, ok := c.Network.ExtraInfo[token].(string)
	if !ok {
		// shit - TODO
	}
	chatRoom := c.chatroomsID[chatRoomID]

	valid := chatRoom.groupPrivateKey.Group.Verify([]byte(chatRoomID), sha1.New(), []byte(answer.ChallengeAnswer))

	if valid {
		// send group private key to user
		groupPriv := pem.EncodeToMemory(&pem.Block{Type: "GROUP PRIVATE KEY", Bytes: chatRoom.groupPrivateKey.Marshal()})

		r := c.Network.Node.GetNode(source)
		if r == nil {
			// shit
		}

		address := *remote.Address
		address.Port = answer.Port

		chatRoom.participants[string(source)] = &address

		msg := validChallengeResponse{chatRoomID, groupPriv, chatRoom.Name, c.port, chatRoom.participants}
		message, _ := kademlia.NewMessage()
		message.MessageType = "GROUP_PRIVATE_KEY"
		message.SourceID = c.Network.Node.ID
		message.Token = token
		message.InsertMessage(msg)

		kademlia.SendMsg(c.Network.Connection, remote.Address, message)

		chatRoom.trustedPeers = append(chatRoom.trustedPeers, r)
		return
	}

	message, _ := kademlia.NewMessage()
	message.MessageType = "FAILED_CHALLENGE"
	message.SourceID = c.Network.Node.ID
	message.Token = token

	kademlia.SendMsg(c.Network.Connection, remote.Address, message)

}

// receiveGroupPrivateKey is a kademlia.ResponseFunc, hence the elaborate signature
// remote is the challenge issuer
// c is the room requester

func (c *client) receiveGroupPrivateKey(remote *kademlia.RemoteNode, token string, source kademlia.NodeID, data interface{}) {
	// this is the last step of all the pingponging.  Hence the cleanup
	defer delete(c.Network.AwaitingResponse, token)
	defer delete(c.Network.ExtraInfo, token)

	response, ok := data.(string)
	if !ok {
		//shit
	}
	asBytes := []byte(response)

	var valid validChallengeResponse
	err := msgpack.Unmarshal(asBytes, &valid)

	if err != nil {
		//shit
	}

	chatRoomIDIface, ok := c.Network.ExtraInfo[token]
	if !ok { // means it's been cleaned up. This request shouldn't have happened
		c.ui <- "...Network error. ExtraInfo has no key. This usually means this is a duplicate request"
		return
	}
	chatRoomID, ok := chatRoomIDIface.(string)
	if !ok {
		panic("chatroomID not a string??!")
	}
	chatRoom, ok := c.chatroomsID[chatRoomID]
	if !ok {
		c.ui <- "...No chatroom found"
		return
	}

	// start decoding and unmarshalling the private key
	block, _ := pem.Decode(valid.PrivateKey)
	if block == nil {
		c.ui <- "Block is not a pem"
		return
	}

	groupPriv, success := chatRoom.groupPrivateKey.Unmarshal(chatRoom.groupPublicKey, block.Bytes)
	if !success {
		c.ui <- "...Unable to succesfully unmarshal private key"
		return
	}

	// apply them to the chatroom
	chatRoom.groupPrivateKey = groupPriv
	chatRoom.Name = valid.Name
	chatRoom.participants = valid.Participants

	// when the participants list is sent from the challenge issuer to the room requester,
	// the challenge issuer's own IP will be 0.0.0.0.
	// this will cause the challenge issuer to not receive the message if a message is sent from the room requester
	// so this fixes that
	sourceNode := c.Network.Node.GetNode(source)
	if sourceNode == nil {
		c.ui <- fmt.Sprintf("...No such remote for source: %#v", source)
		return
	}

	newAddress := *sourceNode.Address
	newAddress.Port = valid.Port

	lA := c.connection.LocalAddr()
	localAddr, _ := lA.(*net.UDPAddr)

	chatRoom.participants[string(source)] = &newAddress

	// fixes  it so that the local node is 0.0.0.0:xxxx - this is an issue only in OS X,  doesn't matter what the self-IP is for linux
	chatRoom.participants[string(c.Node.ID)] = localAddr

	c.chatroomsName[valid.Name] = chatRoom

	// store chatroom ID on kademlia
	c.Network.LocalStore(chatRoom.ID, c.Node.ID)
}

// Generates pem files and stores them in invites/.
func (room *chatroom) GenerateInvite() {
	newMember, err := room.groupPrivateKey.NewMember(rand.Reader)
	if err != nil {
		// shit
	}

	publicFilename := fmt.Sprintf("invites/%s_public.pem", room.ID)
	publicPemFile, err := os.Create(publicFilename)

	if err != nil {
		log.Fatalf("Failed to open %s_public.pem for writing: %s", room.ID, err)
		return
	}
	pem.Encode(publicPemFile, &pem.Block{Type: "GROUP PUBLIC KEY", Bytes: room.groupPrivateKey.Group.Marshal()})
	publicPemFile.Close()

	memberFileName := fmt.Sprintf("invites/%s_member.pem", room.ID)
	memberPemFile, err := os.Create(memberFileName)

	if err != nil {
		log.Fatalf("Failed to open %s_member.pem for writing: %s", room.ID, err)
		return
	}
	pem.Encode(memberPemFile, &pem.Block{Type: "MEMBER PRIVATE KEY", Bytes: newMember.Marshal()})
	memberPemFile.Close()
}
