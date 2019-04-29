package kademlia

import (
	"container/list"
	"crypto/rand"
	"log"
	"net"
	"time"
)

const (
	ID_SIZE     int = 20
	BUCKET_SIZE int = 20
	K               = 8
)

type NodeID []byte

func newNodeID() NodeID {
	b := make([]byte, ID_SIZE)
	if _, err := rand.Read(b); err != nil {
		log.Fatalln("Unable to get random node ID. Error was: ", err)
	}

	// var n [ID_SIZE]byte
	// copy(n[:], b[:ID_SIZE])

	return NodeID(b)
}

func newEmptyNodeID() NodeID {
	return NodeID(make([]byte, ID_SIZE))
}

func (id NodeID) String() string {
	// return string(id[:])
	return string(id)
}

func (id NodeID) DistanceTo(cmp NodeID) NodeID {
	res := newEmptyNodeID()
	log.Printf("CMP Length: %d %d %d\n", len(cmp), len(res), len(id))
	for i := 0; i < ID_SIZE; i++ {
		res[i] = id[i] ^ cmp[i]
	}
	return res
}

func (id NodeID) EqualsTo(cmp NodeID) bool {
	for i := 0; i < ID_SIZE; i++ {
		if id[i] != cmp[i] {
			return false
		}
	}
	return true
}

func (id NodeID) LessThan(cmp NodeID) bool {
	for i := 0; i < ID_SIZE; i++ {
		if id[i] < cmp[i] {
			return true
		}
	}
	return false
}

func (id NodeID) GetBucketID() int {
	for i := 0; i < ID_SIZE; i++ {
		for j := 7; j >= 0; j-- {
			if (id[i] >> uint8(j) & 0x1) != 0 { // TODO: check if bitshift makes sense
				return ID_SIZE*8 - 1 - (i*8 + 7 - j)
			}
		}
	}
	return ID_SIZE*8 - 1
}

// a routingTable is just the buckets as per the original kademlia spec
// we use the "list" package because it's a double linked list, saves time from having to write one myself
// each routing table has an array of list, one list for each bit.
// using a double linked list allows for a very easy update function for the routing table
type routingTable [ID_SIZE * 8]*list.List

func newRoutingTable() *routingTable {
	var rt routingTable
	for i := 0; i < ID_SIZE*8; i++ {
		rt[i] = list.New()
	}
	return &rt
}

type RemoteNode struct {
	ID            NodeID
	Address       *net.UDPAddr
	lastResponded time.Time
	verifiedBy    []*RemoteNode

	hasKey map[string]bool
}

func newRemoteNode(ID NodeID, addr *net.UDPAddr) *RemoteNode {
	return &RemoteNode{
		ID:         ID,
		Address:    addr,
		verifiedBy: make([]*RemoteNode, 0),

		hasKey: make(map[string]bool),
	}
}

type Node struct {
	ID            NodeID
	IP            string
	Port          int
	RoutingTable  *routingTable
	AddressToNode map[string]*RemoteNode

	Store map[string]interface{}
}

func NewNode() *Node {
	return &Node{
		ID:            newNodeID(),
		RoutingTable:  newRoutingTable(),
		AddressToNode: make(map[string]*RemoteNode),

		Store: make(map[string]interface{}),
	}
}

// R
func (node Node) GetNodeFromAddress(address string) (remote *RemoteNode, exists bool) {
	if address == "" {
		panic("No Address")
	}

	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		panic("Insert BAD SHIT here")
	}

	if addr.String() == "" {
		panic("Something went wrong, and resolution failed")
	}

	remote, exists = node.AddressToNode[addr.String()]
	return
}

// C & U
func (node Node) Update(cmp *RemoteNode) {
	bucketID := node.ID.DistanceTo(cmp.ID).GetBucketID()
	bucket := node.RoutingTable[bucketID]

	var found bool = false
	var foundElement *list.Element

	for elem := bucket.Front(); elem != nil; elem = elem.Next() {
		e, ok := elem.Value.(*RemoteNode)
		if !ok {
			continue // if it's not a NodeID, wtf is it doing in the list?? Probably should error out
		}
		if e.ID.EqualsTo(cmp.ID) || e.ID.EqualsTo(node.ID) {
			found = true
			foundElement = elem
			break
		}
	}
	if !found {
		if bucket.Len() <= BUCKET_SIZE {
			bucket.PushFront(cmp)
		}
	} else {
		foundElement.Value = cmp // update the  foundElement value
		bucket.MoveToFront(foundElement)
	}
}

func (node Node) GetOrCreateNode(id NodeID, address string) (remote *RemoteNode) {
	remote, exists := node.GetNodeFromAddress(address)

	if exists {
		return remote
	}

	addr, err := net.ResolveUDPAddr("udp", address)

	if err != nil {
		panic("Shit")
	}

	remote = newRemoteNode(id, addr)
	node.AddressToNode[addr.String()] = remote
	node.Update(remote)

	return
}

func (node Node) GetNode(id NodeID) (remote *RemoteNode) {
	bucketID := node.ID.DistanceTo(id).GetBucketID()
	bucket := node.RoutingTable[bucketID]
	for elem := bucket.Front(); elem != nil; elem = elem.Next() {
		e, ok := elem.Value.(*RemoteNode)
		if !ok {
			continue
		}
		if string(e.ID) == string(id) {
			return e
		}
	}
	return nil
}

// D
func (node Node) Delete(remote *RemoteNode) {
	delete(node.AddressToNode, remote.Address.String())
	bucketID := remote.ID.DistanceTo(node.ID).GetBucketID()
	bucket := node.RoutingTable[bucketID]
	for elem := bucket.Front(); elem != nil; elem = elem.Next() {
		e, ok := elem.Value.(*RemoteNode)
		if !ok {
			continue
		}
		if e == remote {
			bucket.Remove(elem)
		}
	}
}

func (node Node) GetNClosestNodes(target NodeID, n int) []*RemoteNode {
	bucketID := target.DistanceTo(node.ID).GetBucketID()
	bucket := node.RoutingTable[bucketID]

	var retVal []*RemoteNode

	for elem := bucket.Front(); elem != nil; elem = elem.Next() {
		e, ok := elem.Value.(*RemoteNode)
		if !ok {
			continue // TODO: proper erroring out
		}

		retVal = append(retVal, e)
	}

	if len(retVal) < n {
		// then we need more. so we iterate through the rest of the buckets
		goto lookaheadandback
	} else {
		if len(retVal) > n {
			retVal = retVal[:n] // trim to the required size
		}
		return retVal
	}

	// this goto block is just for organizational purposes
lookaheadandback:
	for i := 1; (len(retVal) < n) && (bucketID-i >= 0 || bucketID+i < ID_SIZE*8); i++ {
		if bucketID+i < ID_SIZE*8 {
			// look ahead i buckets

			lookAheadBucket := node.RoutingTable[bucketID+i]
			for elem := lookAheadBucket.Front(); elem != nil; elem = elem.Next() {
				e, ok := elem.Value.(*RemoteNode)
				if !ok {
					continue // proper errors plz kthxbai
				}
				retVal = append(retVal, e)
			}
		}

		// we prioritize lookaheads over lookbacks
		if len(retVal) > n {
			break
		}

		if bucketID-i >= 0 {
			// look back i buckets

			lookBackBucket := node.RoutingTable[bucketID-i]
			for elem := lookBackBucket.Front(); elem != nil; elem = elem.Next() {
				e, ok := elem.Value.(*RemoteNode)
				if !ok {
					continue // proper errors plz kthxbai
				}

				retVal = append(retVal, e)
			}
		}
	}
	if len(retVal) > n {
		retVal = retVal[:n]
	}
	return retVal
}

func (node Node) GetClosestNodes(n int) []*RemoteNode {
	var resVal []*RemoteNode
	for _, bucket := range node.RoutingTable {
		for elem := bucket.Front(); elem != nil; elem = elem.Next() {
			e, ok := elem.Value.(*RemoteNode)
			if !ok {
				continue
			}
			resVal = append(resVal, e)
		}

		if len(resVal) == n {
			break
		}
	}

	return resVal
}

func (node Node) GetNearestNode() *RemoteNode {
	for i, bucket := range node.RoutingTable {
		log.Println("Bucket #", i, bucket)
		for elem := bucket.Front(); elem != nil; elem = elem.Next() {
			log.Println(elem.Value)
			e, ok := elem.Value.(*RemoteNode)
			if !ok {
				continue
			}
			return e
		}
	}
	return nil
}

// spring clean basically purges the AddressToNode map, and refills it.
// spring cleaning should ideally happen every 10 minutes or so
func (node Node) SpringClean() {
	node.AddressToNode = make(map[string]*RemoteNode)
	for _, bucket := range node.RoutingTable {
		for elem := bucket.Front(); elem != nil; elem = elem.Next() {
			e, ok := elem.Value.(*RemoteNode)
			if !ok {
				bucket.Remove(elem)
			}
			node.AddressToNode[e.Address.String()] = e
		}
	}
}
