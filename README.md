This server will implement a mix-net for contact tracing, to hide the identities of users when they broadcast infection status to recent contacts.

It includes three separate components:

1. Blinding server: using Node.js for endpoints and a super basic libsodium implementation of the exponentiations.
    1. Endpoints:
        1. Post: we need an endpoint for users to post their tokens to be shuffled
        2. Poll: Since the shuffling might be slow during high load, on Post, we simply return a unique URL for the user to poll to retrieve the shuffled tokens.
    2. Blinding plugin. We need a highly performant (C++ + Libsodium?) plugin that implements a single function:
        1. Shuffle
        2. Exponentiate with secret exponent. (Perhaps this secret token should rotate for more forward security?)

2. Mixnet forwarder: all it would do is take transformed tokens, batch them up, and forward the batch onto the next server in a chain, who would do the same thing. You need at least two of these connected in a chain. Each mixnet unwraps one layer of onion encryption, and then shuffles.
    1. Endpoints:
        1. Post: this can be used by either a user or the upstream mixnet forwarder.
        2. Get public key: just a static ascii-armored public key.
    2. Pushing:
        1. The forwarder will wait a specified amount of time, or until it has batched and permuted sufficient number of tokens before pushing data to the next mixnet forwarder (or to the database store).

3. Database store: : all it does is accept tokens and puts them into a database and/or Bloom filter, and then make the entire thing available to download.
    1. Endpoints:
        1. Post: this will be used by the upstream mixnet forwarder to send tokens to the store.
        2. Get: just a static download allowing users to download either the full encrypted database, or a Bloom filter of them.
