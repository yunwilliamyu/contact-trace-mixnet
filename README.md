This server will implement a mix-net for contact tracing, to hide the identities of users when they broadcast infection status to recent contacts.

It includes three separate components:

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
This may either receive messages from end-users who are self-reporting, or from the previous node in the chain.

The request will contain:

```
actual_request: []bytes   # a binary blob containing a collection of NaCL boxes (the onion packets) concatenated together
```


## Endpoints

### Post

This can be used by either a user or the upstream mixnet forwarder.

### Get public key

Just a static ascii-armored public key.

## Pushing:
The forwarder will wait a specified amount of time, or until it has batched and permuted sufficient number of tokens before pushing data to the next mixnet forwarder (or to the database store).

# Database store

All it does is accept tokens and puts them into a database and/or Bloom filter, and then make the entire thing available to download.

## Endpoints:

### Post

This will be used by the upstream mixnet forwarder to send tokens to the store.

### Get

Just a static download allowing users to download either the full encrypted database, or a Bloom filter of them.
