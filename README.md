This server will implement a mix-net for contact tracing, to hide the identities of users when they broadcast infection status to recent contacts.

It includes three separate components:

# Blinding server

Using Go and libsodium.

## Endpoints:

### Post
We need an endpoint for users to post their tokens to be shuffled.

The request will contain:

```
actual_request {
  values: []string
  day: date # chooses the key
}
mobile_os: enum
signed_nonce: string # hash of actual request signed by mobile OS's "device verification" mechanism
```

The response will contain:

```
values: []string
```

# Mixnet forwarder

All it would do is take transformed tokens, batch them up, and forward the batch onto the next server in a chain, who would do the same thing. You need at least two of these connected in a chain. Each mixnet unwraps one layer of onion encryption, and then shuffles.

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
