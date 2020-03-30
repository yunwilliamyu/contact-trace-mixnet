This server will implement a mix-net for contact tracing, to hide the identities of users when they broadcast infection status to recent contacts.

It includes three separate components:

Blinding server: using Node.js for endpoints and a super basic libsodium implementation of the exponentiations.

Mixnet forwarder: all it would do is take transformed tokens, batch them up, and forward the batch onto the next server in a chain, who would do the same thing. (You need at least two of these)

Database store: : all it does is accept tokens and puts them into a Bloom filter, and then make the entire thing available to download.
