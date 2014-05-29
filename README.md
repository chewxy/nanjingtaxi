nanjingtaxi
===========

Nanjing Taxi is a relatively secure distributed P2P chat system. It uses a distributed hash table in the form of a Kademlia network to "host" chatrooms. Clients connect to the network to find a chatroom. Once a client finds its chatroom, the connections in the chatroom is separate from the Kademlia network.

The way Nanjing Taxi works is like this:

1. Client connects to a Kademlia network
2. Client joins/creates a room
3. If client creates a room:
    a. Client will store the room ID in the Kademlia network, allowing the room to be found
    b. Client will generate keys (public, private and member)
    c. Clent will facilitate key exchange
4. If client joins a room:
    a. Client will be provided with a member key and a public key by someone trusted
    b. Client will request a room from a node in the Kademlia network that is part of the room
    c. Said node will issue a challenge to client.
    d. Client will respond to the challenge
    e. If response is valid, the room's private key is distributed to the Client, allowing Client to create invites.

##Explanation In Images##
![first](http://i.imgur.com/dImpsiA.jpg)

This is how the network looks like after a node joins the Kademlia network. We're the blue node on the lower right. Blue nodes are participants of a chatroom.

![alt tag](http://i.imgur.com/c09vHLX.jpg)

This is how the network looks like after the node requests a chatroom. All the nodes in the chatroom are now connected separately from the Kademlia network.


##To Use##

```
user@host: ~/location/of/project$ go build .
user@host: ~/location/of/project$ ./nanjingtaxi <kademlia port> <chatroom port>
```

These are the commands available. Follow the prompts after typing in the commands

* **cx** - connect to a kademlia network
* **nodes** - check the nodes in the kademlia network
* **self** - introspection. See stuff about this node
* **ls** - list the number of chatrooms this client is in
* **new** - create a new chatroom
* **join** - join a chatroom
* **invite** - create invite to a chatroom
* **send** - send message to a chatroom

##Limitations##

* I don't think the private key transmission is too secure
* Works on simple LANs. Untested on more complex network structures.
* No UDP firewall punching, NAT traversal and the like
* Crappy interface.
* Doesn't have an indication of who sent the message, just the message (easily rectified).

##Misc##

* **Why is it called Nanjing Taxi?** It's because I was inspired by a rather ridiculous taxi driver who juggled 4 different communication networks while driving me in Nanjing. I wrote a write up to this app here: [The Nanjing Taxi](http://blog.chewxy.com/2014/05/30/the-nanjing-taxi/)
* **Why do you think this is cool?** I like distributed stuff. The lack of a need for a central server? No logins? What's not to like.
* **Why is this useful** I don't know. Do you ever have a need to securely communicate with other people?
* **Will there be improvements to this? Like NAT traversal and stuff? You know, to make it useful?** I am not sure and I cannot commit to a schedule. My life is kinda hectic right now. Feel free to send a pull request. 
* **Your code sucks**, well, I wrote it in a hotel room during my holiday. It's a hackjob.
