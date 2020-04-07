This server will implement a mix-net for contact tracing, to hide the identities of users when they broadcast infection status to recent contacts.

It includes three separate components: (1) a blinding server, which serves to prevent an after-the-fact dictionary attack on geohashes, (2) a mix-net, which protects against social graph attack by the data store, and (3) a data store server, whch actually hosts the encrypted messages between uesers in a series of deaddrops (mailboxes).

# Blinding server

Using Go and libsodium.

## Endpoints:

### Post
We have an endpoint for users to post their tokens to be shuffled.

The request will contain:

```
actual_request {
  "DayID": date         # integer; chooses the key
  "Inputs": []string    # a byte-string containing all the 32-byte tokens as ECC curve points concatenated together, encoded in hexadecimal
}
mobile_os: enum
signed_nonce: string # hash of actual request signed by mobile OS's "device verification" mechanism
```

The response will contain:

```
values: []string
```

# Mixnet forwarder

We have implemented a batched forward-only node of a linear mix-net. By having a linear system, we are able to cut down on the overhead of including routing information in the onion packets.

Upon receipt of a message containing a collection of onion packets, the node unwraps one layer of the onion packets, and stores them temporarily.
When the number of messages (or the time between pushes) reaches some threshold, the node batches all the onion packets together, shuffles them, and pushes them to the next node in the mixnet chain.

In order to have any privacy, you need at least two of these nodes connected in a chain, but the more the better.
Privacy of the individual sending messages through the chain is maintained so long as 1-of-n nodes are honest.

## Endpoints:

### Post
This may either receive messages from end-users, or from the previous node in the chain.

The request will contain:

```
actual_request: []bytes   # a binary blob containing a collection of NaCL boxes (the onion packets) concatenated together
```

## Pushing:
The forwarder will wait a specified amount of time, or until it has batched and permuted sufficient number of tokens before pushing data to the next mixnet forwarder (or to the database store).

The push format is the same as the format in which the forwarder received messages, but with one fewer layer of onion encryption.
Note for implementation purposes that this means the out-going messages will be slightly smaller than the incoming messages.
Each mixnet node thus must know its position in the linear chain, so that it knows how long the messages it expects to receive are.

## Replicas:
You can turn up as many replicas of each mix-node as desired, so long as they all have the same keyset.
The upstream node's message can be processed by any of them, though note that each replica will have to wait until it reaches the threshold number of onion packets before it pushes to the next stage of the mixnet.
Turning up too many replicas may increase latency, but this is easily avoided by only turning up replicas if a stage of the mix-net is reaching capacity.

# Database store v1 (with forwarding; 1-of-2 privacy)
The database accepts (mostly) unwrapped onion packets from the final mix-net node, and stores them in a database (format to be determined).
This database can be thought of as a collection of mailboxes, with possibly multiple messages per address, stored in order of receipt.
A query to the database takes the form of asking for all messages in a particular mailbox that arrived after a specific message.

Furthermore, this version of the database also records a collection of forwarding addresses on the polling gateway for each mailbox, along with a subscription marker.
Periodically, the database will send all mail that arrived after the subscription marker to a forwarding address.
It will then update the subscription marker to the current state of the mailbox.

## Endpoints:
The database store also receives binary blobs in the same format as the mix-net servers, and unwraps the final onion encryption wrapping, to reveal actual envelopes.

### Post
```
actual_request: []bytes   # a binary blob containing a collection of NaCL boxes (the onion packets) concatenated together
```

This will be used by the upstream mixnet forwarder to send tokens to the store.

## Message format
Although the endpoint receives a collection of NaCL boxes, there are several different types of wrapped messages that the Database Store needs to process:

1. Publish request: An encrypted message to be deposited in a mailbox for retrieval.
```
mailbox addr + box(encrypted message)
```

2. Sub request: A request to set up a forwarding address.
```
mailbox addr + box(polling gw mailbox addr)
```
Note that the polling gw mailbox addr is actually just another box(dead drop id + distinguisher).

The database store will send messages to the polling gateway with the following format
```
box(dead drop id + distinguisher) + forwarded message
```


# Polling gateway v1 (1-of-2 privacy)
The polling gateway acts as an agent for Alice to retrieve mailboxes from the Database Store v1.
To that end, Alice registers a mailbox with N ephemeral receiving addresses.
Then, Alice, sends a series of N messages to the Database Store through the mixnet, each one associating a Database Store address with an ephemeral receiving address on the Polling gateway.

When the polling gateway receives a message from the Database Store, it will include an encrypted message, as well as a box(dead drop id + distinguisher).
The polling gateway will unwrap the box, and put the encrypted message along with the distinguisher into the dead drop

## Endpoints:
We only need to allow Alice to poll the dead drop. Since we assume that dead drops are unique to a person, once the dead drop has been polled, the Polling Gateway can discard all messages in the dead drop.

### poll (exposed to users)
```
dead_drop_id: []bytes
last_read_message_hash: []bytes // truncated
```

We also need a starting token, to indicate which messages were already read (we should ensure that a failed poll request doesn't drop messages on the floor). We can remove old messages when we get the next poll request, that indicates that these messages were already processed.


# Round-trip Mix-net node (v2) (for 1-of-n privacy)
The v1 polling gateway and database store are limited to 1-of-2 privacy. This is due to the fact that the polling gateway and database store can collude to determine the source IP addresses of queries to any set of mailboxes. Even if the database store forwards messages through a mix-net, it can choose to send a specially crafted message designed to reveal the shuffling and address obfuscation when received by the polling gateway. Thus, there is not benefit to using the mix-net for forwarding messages. This is in stark contrast to the messages sent to the database store, which instead have 1-of-n privacy, where so long as there is one honest mix server, privacy for the sender is preserved. In v1, privacy for the recipient requires at least 1 of the database store or the polling gateway to be honest.

Preventing this requires a mix-net structure and message format that allows messages round-trip messages. This has not yet been implemented, but by performing a deterministic address re-encryption (see Chaum), we can implement the equivalent of a polling gateway for the recipient, without allowing collusion of the polling gateway and database store to reveal recipient IP addreses.
