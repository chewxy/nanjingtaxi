package kademlia

import (
	"net"
	"time"

	"fmt"
	"log"

	"github.com/vmihailenco/msgpack"
)

type Kademlia struct {
	Node       *Node
	Name       string // this is used as the chatroom ID
	Connection *net.UDPConn

	packets  chan packet
	requests chan Message
	kill     chan bool

	ResponseHandler map[string]ResponseFunc

	AwaitingResponse map[string]time.Time   // key is token - req/rep method
	ExtraInfo        map[string]interface{} // key is token. This is a store of random things that may be needed

	pendingQueries   map[string]time.Time   // key is token - this represents a queue of sorts for work
	pendingEnvelopes map[string]*RemoteNode // key is token - this is for all the return addresses

	ResultChan map[string]chan interface{} // key is token

}

func NewKademlia() *Kademlia {
	k := &Kademlia{
		Node: NewNode(),

		packets:  make(chan packet),
		requests: make(chan Message),
		kill:     make(chan bool),

		ResponseHandler: make(map[string]ResponseFunc),

		AwaitingResponse: make(map[string]time.Time),
		ExtraInfo:        make(map[string]interface{}),

		pendingQueries:   make(map[string]time.Time),
		pendingEnvelopes: make(map[string]*RemoteNode),

		ResultChan: make(map[string]chan interface{}),
	}

	k.ResponseHandler = map[string]ResponseFunc{
		"PING":                k.pong,
		"PONG":                k.pongResponse,
		"FIND_NODE":           k.findNodeResponse,
		"FIND_NODE_RESPONSE":  k.findNodeResponseHandler,
		"STORE":               k.storeResponse,
		"STORE_RESPONSE":      k.storeResponseHandler,
		"FIND_VALUE":          k.findValueResponse,
		"FIND_VALUE_RESPONSE": k.findValueResponseHandler,
	}

	return k
}

func (dht *Kademlia) Run() {
	// start server
	// bootstrap network
	// listen for DHT messages
	if dht.Connection == nil {
		dht.initNetwork()
	}

	go dht.readFromSocket()
	go dht.processPackets()
	go dht.handleMessages()
	for {
		select {
		case <-dht.kill:
			return
		default:
			continue
		}
	}

}

type ResponseFunc func(*RemoteNode, string, NodeID, interface{})

func (dht *Kademlia) Ping(remote *RemoteNode) string {
	message, token := NewMessage()
	message.MessageType = "PING"
	message.SourceID = dht.Node.ID

	SendMsg(dht.Connection, remote.Address, message)

	// register token with AwaitingResponse
	dht.AwaitingResponse[token] = time.Now()
	return token
}

func (dht *Kademlia) PingIP(addr *net.UDPAddr) string {
	message, token := NewMessage()
	message.MessageType = "PING"
	message.SourceID = dht.Node.ID

	SendMsg(dht.Connection, addr, message)

	dht.AwaitingResponse[token] = time.Now()
	return token
}

func (dht *Kademlia) pong(remote *RemoteNode, token string, source NodeID, data interface{}) {
	message, _ := NewMessage() // token is not needed
	message.MessageType = "PONG"
	message.SourceID = dht.Node.ID
	message.Token = token

	SendMsg(dht.Connection, remote.Address, message)
}

func (dht *Kademlia) pongResponse(remote *RemoteNode, token string, source NodeID, data interface{}) {
	defer delete(dht.AwaitingResponse, token)

	remote.lastResponded = time.Now()
	dht.Node.AddressToNode[remote.Address.String()] = remote // store it in the index
}

//LocalStore basically stores data in the local node
func (dht *Kademlia) LocalStore(key string, value interface{}) {
	dht.Node.Store[key] = value
}

func (dht *Kademlia) Store(remote *RemoteNode, key string, value interface{}) string {
	msg := map[string]interface{}{key: value}

	message, token := NewMessage()
	message.MessageType = "STORE"
	message.SourceID = dht.Node.ID
	message.InsertMessage(msg)

	SendMsg(dht.Connection, remote.Address, message)
	dht.AwaitingResponse[token] = time.Now()
	return token
}

func (dht *Kademlia) storeResponse(remote *RemoteNode, token string, source NodeID, data interface{}) {
	p, _ := data.(string)
	byteArrayP := []byte(p)

	var m map[string]interface{}
	msgpack.Unmarshal(byteArrayP, &m)

	for k, v := range m {
		dht.LocalStore(k, v)
	}

	message, _ := NewMessage()
	message.MessageType = "STORE_RESPONSE"
	message.SourceID = dht.Node.ID
	message.Message = "OK"

	SendMsg(dht.Connection, remote.Address, message)
}

func (dht *Kademlia) storeResponseHandler(remote *RemoteNode, token string, source NodeID, data interface{}) {
	defer delete(dht.AwaitingResponse, token)
	remote.lastResponded = time.Now()
}

func (dht *Kademlia) FindNode(remote *RemoteNode, cmp NodeID) string {
	message, token := NewMessage()
	message.MessageType = "FIND_NODE"
	message.SourceID = dht.Node.ID
	message.Message = cmp

	SendMsg(dht.Connection, remote.Address, message)
	// register awaiting response with token
	dht.AwaitingResponse[token] = time.Now()
	// register additional data with token
	dht.ExtraInfo[token] = cmp
	// register a channel
	dht.ResultChan[token] = make(chan interface{})

	return token
}

func (dht *Kademlia) findNodeResponse(remote *RemoteNode, token string, source NodeID, data interface{}) {
	t, ok := data.(string)
	if !ok {
		panic(fmt.Sprintf("SHIT. params is %T | %#v\n", data, data))
	}

	target := NodeID(t)
	closestNodes := dht.Node.GetNClosestNodes(target, 8)

	message, _ := NewMessage()
	message.MessageType = "FIND_NODE_RESPONSE"
	message.SourceID = dht.Node.ID
	message.InsertMessage(closestNodes)
	// replace the autogenerated token  with the received token so the sender knows which message this is replying to
	message.Token = token

	SendMsg(dht.Connection, remote.Address, message)
}

func (dht *Kademlia) findNodeResponseHandler(remote *RemoteNode, token string, source NodeID, data interface{}) {
	defer delete(dht.AwaitingResponse, token)
	defer delete(dht.ExtraInfo, token)
	defer func() { remote.lastResponded = time.Now() }()

	// handle the data received
	p, _ := data.(string)
	byteArrayT := []byte(p)
	var remoteNodes []*RemoteNode
	msgpack.Unmarshal(byteArrayT, &remoteNodes)

	t, ok := dht.ExtraInfo[token]
	if !ok {
		// do something
		panic("Nothing found in ExtraInfo. Weird")
	}

	target, ok := t.(NodeID)
	if !ok {
		panic(fmt.Sprintf("t is supposed to be a NodeID. It is %T instead", t))
	}

	// here we use a simple trick - not scalable for larger scale DHTs, but for small networks it works well
	// basically it looks for the remote nodes that this node already knows about.
	// the problem starts coming in when you have more than 160 nodes I guess.
	var unseen []*RemoteNode = make([]*RemoteNode, 0)
	for _, r := range remoteNodes {
		if string(r.ID) == string(dht.Node.ID) {
			continue // skip anything that is itself
		}

		//check if node already exists. If not, send a new message with the target in mind
		_, seen := dht.Node.AddressToNode[r.Address.String()]

		if !seen {
			unseen = append(unseen, r)
		}

		if string(r.ID) == string(target) {
			ch, ok := dht.ResultChan[token]
			if !ok {
				panic("No Result chan available to send the resulting node to chan")
			}
			ch <- r

			return // bailout
		}
	}

	for _, remote := range unseen {
		dht.FindNode(remote, target)
	}
	log.Println("Done with Find Node Response Handler")
	return

}

func (dht *Kademlia) FindValue(remote *RemoteNode, key string) string {
	message, token := NewMessage()
	message.MessageType = "FIND_VALUE"
	message.SourceID = dht.Node.ID
	message.Message = key

	SendMsg(dht.Connection, remote.Address, message)
	// register awaiting response with token
	dht.AwaitingResponse[token] = time.Now()
	dht.ExtraInfo[token] = key
	dht.ResultChan[token] = make(chan interface{})

	log.Println("Sending FIND_VALUE. Token is: ", token)

	return token

}

func (dht *Kademlia) findValueResponse(remote *RemoteNode, token string, source NodeID, data interface{}) {
	key, ok := data.(string)
	if !ok {
		panic(fmt.Sprintf("SHIT. params is %T | %#v\n", data, data))
	}

	msgType := "FIND_VALUE_RESPONSE"

	value, ok := dht.Node.Store[key]
	if !ok {
		value = dht.Node.GetClosestNodes(8)
		// msgType = "FIND_VALUE_RESPONSE_ITERATIVE"
	}

	message, _ := NewMessage()
	message.MessageType = msgType
	message.SourceID = dht.Node.ID
	message.InsertMessage(value)
	message.Token = token

	SendMsg(dht.Connection, remote.Address, message)

}

func (dht *Kademlia) findValueResponseHandler(remote *RemoteNode, token string, source NodeID, data interface{}) {
	defer delete(dht.AwaitingResponse, token)
	defer delete(dht.ExtraInfo, token)
	defer func() { remote.lastResponded = time.Now() }()

	log.Println("IN FIND_VALUE_RESPONSE. Token is ", token)

	// get target info
	t, ok := dht.ExtraInfo[token]
	if !ok {
		// do something
		panic("Nothing found in ExtraInfo. Weird")
	}
	target, ok := t.(string) // we can do this because all the keys are strings
	if !ok {
		panic("target key is supposed to be a string")
	}

	// unmarshal
	p, _ := data.(string)
	byteArrayP := []byte(p)
	var remoteNodes []*RemoteNode
	err := msgpack.Unmarshal(byteArrayP, &remoteNodes)

	if err != nil {
		// it's not a list of remotenodes - it's a nodeID
		var nid NodeID
		nidUnmarshallErr := msgpack.Unmarshal(byteArrayP, &nid)

		if nidUnmarshallErr != nil {
			log.Fatalf("data is not a nodeID. It is %#v(%T) Error: %s", data, data, nidUnmarshallErr)
			return
		}

		r := dht.Node.GetNode(nid)

		if r == nil {
			panic("remote not found")
		}

		r.hasKey[target] = true

		// this means it's not a list of remote nodes.
		ch, ok := dht.ResultChan[token]
		if !ok {
			panic("No Return Result Channel")
		}

		ch <- byteArrayP
		return
	}

	log.Println("Not found. Looking Iteratively")

	// if a list of remoteNodes is returned, that means this remote node doesn't have the key
	rN := dht.Node.GetNode(source)
	rN.hasKey[target] = false

	var unseen []*RemoteNode = make([]*RemoteNode, 0)
	for _, r := range remoteNodes {

		hasKey, ok := r.hasKey[target]
		if !ok {
			unseen = append(unseen, r)
			continue
		}
		if !hasKey {
			// has key but value = false
			continue
		}

		// else, query again to see if this remote really has it
		unseen = append(unseen, r)

	}

	for _, r := range unseen {
		dht.FindValue(r, target)
	}

}

func (dht *Kademlia) getPeers(remote *RemoteNode, params ...interface{}) {

}

func (dht *Kademlia) getPeersResponse(remote *RemoteNode, token string, source NodeID, data interface{}) {

}
