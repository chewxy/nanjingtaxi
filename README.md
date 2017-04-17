nanjingtaxi
===========

Nanjing Taxi is a relatively secure distributed P2P chat system. It uses a distributed hash table in the form of a Kademlia network to "host" chatrooms. Clients connect to the network to find a chatroom. Once a client finds its chatroom, the connections in the chatroom is separate from the Kademlia network.

I have been made to understand that this system is extremely similar to the one proposed by the BitTorrent team - BitTorrent Chat. The [basic idea](http://blog.bittorrent.com/2013/12/19/update-on-bittorrent-chat/) is similar.

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

## Explanation In Images ##
![first](http://i.imgur.com/dImpsiA.jpg)

This is how the network looks like after a node joins the Kademlia network. We're the blue node on the lower right. Blue nodes are participants of a chatroom.

![alt tag](http://i.imgur.com/c09vHLX.jpg)

This is how the network looks like after the node requests a chatroom. All the nodes in the chatroom are now connected separately from the Kademlia network.


## To Use ##

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

### Typical Flow ###

If you're creating a room:

1. `./nanjingtaxi 13370 12345` - 13370 is the port that will be used to connect to Kademlia. 12345 is the communications port.
2. `cx` - issues a connection command. A prompt for the target IP will come up. You need to know an IP:Port combination that is already on the Kademlia network
3. `new` - creates a new room. It will prompt you for a user friendly name for the room. Then it will generate 3 keys: **chatrooms/<roomID>_public.pem**, **chatrooms/<roomID>_private.pem** and **keys/<roomID>_member.pem**. These keys are used for challenge-replies
4. To invite people to the room, `invite`. It will generate 2 keys: **invites/<roomID>_member.pem** and **invites/<roomID>_public.pem**. Distribute this key to the person you're inviting (preferably in a secure manner).

If you're joining a room:

1. `./nanjingtaxi 13370 12345`
2. `cx`
3. Place the given **<roomID>_public.pem** file in the `chatrooms/` directory. **<roomID>_member.pem** is to be placed in `keys/`.
4. `join` - supply the room ID, authentication will be done and you'll be ready to chat.

To chat:

1. `send` - follow the prompts, enter the room ID.

### Room ID ###

Room IDs are UUID4s.


## Tested On ##

* Ubuntu 12.04
* OS X 10.9
* Ubuntu 13.10
* Ubuntu 14.04

## Limitations ##

* I don't think the private key transmission is too secure
* Works on simple LANs. Untested on more complex network structures.
* No UDP firewall punching, NAT traversal and the like
* Crappy interface.
* Doesn't have an indication of who sent the message, just the message (easily rectified).

## Misc ##

* **Why is it called Nanjing Taxi?** It's because I was inspired by a rather ridiculous taxi driver who juggled 4 different communication networks while driving me in Nanjing. I wrote a write up to this app here: [The Nanjing Taxi](http://blog.chewxy.com/2014/05/30/the-nanjing-taxi/)
* **Why do you think this is cool?** I like distributed stuff. The lack of a need for a central server? No logins? What's not to like.
* **Why is this useful?** I don't know. Do you ever have a need to securely communicate with other people?
* **Will there be improvements to this? Like NAT traversal and stuff? You know, to make it useful?** I am not sure and I cannot commit to a schedule. My life is kinda hectic right now. Feel free to send a pull request. 
* **Your code sucks**, well, I wrote it in a hotel room during my holiday. It's a hackjob. Of course there are no tests.

## Open Source Stuff ##
This code is open source. Please feel free to hack on it, and if you want to contribute, send a pull request. It's MIT licenced.
