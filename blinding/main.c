#include <sodium.h> // This must be libsodium 1.0.18+
#include <time.h>

// None of the code in main is meant to be run. 
// It is just here to test out the various functions which will be
// performed by the client vs server.

// We will be using Ristretto (this code is based closely off of
// https://libsodium.gitbook.io/doc/advanced/point-arithmetic/ristretto

int print_ristretto(const unsigned char a[], const size_t length);


int print_ristretto(const unsigned char a[], const size_t length) {
    int i;
    for (i=0; i<length; i++) {
        printf("%02X", a[i]);
    }
    printf("\n");
}


// Generates a safe permutation
int safe_permutation(size_t perm[], size_t L) {
	for (int i = 0; i < L; i++) perm[i] = i;
	for (int i = 0; i < L; i++) {
		int j, t;
		j = randombytes_random() % (L-i) + i;
		t = perm[j]; perm[j] = perm[i]; perm[i] = t; // Swap i and j
	}
	return 0;
}

// B = output array of exponentiated tokens
// A = input array of tokens
// k = secret exponent
// length = number of tokens
int exponentiate(unsigned char B[][crypto_core_ristretto255_BYTES], unsigned const char A[][crypto_core_ristretto255_BYTES], unsigned const char k[], const size_t length) {
    for (int i=0; i<length; i++)
    {
        crypto_scalarmult_ristretto255(B[i], k, A[i]);
    }
}

// B = output array of shuffled tokens
// A = input array of tokens
// length = number of tokens
int shuffle(unsigned char B[][crypto_core_ristretto255_BYTES], unsigned const char A[][crypto_core_ristretto255_BYTES], const size_t length) {
    size_t I[length];
    safe_permutation(I, length);
    //for (int i=0; i<length; i++) printf("%i ",I[i]);
    for (int i=0; i<length; i++) {
        for (int j=0; j<32; j++) {
            B[I[i]][j]=A[i][j];
        }
    }
}

// Boths shuffles and exponentiates
int shuffle_and_blind(unsigned char B[][crypto_core_ristretto255_BYTES], unsigned const char A[][crypto_core_ristretto255_BYTES], unsigned const char k[], const size_t length) {
    unsigned char C[length][crypto_core_ristretto255_BYTES];
    shuffle(C, A, length);
    exponentiate(B, C, k, length);
}

// Helper function so I can get a few deterministic keys (never should be used in real implementation
int key(unsigned char sec[], const int n) {
    //unsigned char sec[crypto_core_ristretto255_BYTES];
    //crypto_core_ristretto255_random(sec);
    unsigned char bytes[4];
    bytes[0] = (n >> 24) & 0xFF;
    bytes[1] = (n >> 16) & 0xFF;
    bytes[2] = (n >> 8) & 0xFF;
    bytes[3] = n & 0xFF;
    crypto_generichash(sec, 64, bytes, 4, NULL, 0);
}


int main(void)
{
    if (sodium_init() < 0) {
        /* panic! the library couldn't be initialized, it is not safe to use */
    }

    int num_tokens = 50000;
    int num_print = 10;

    // Let's get some deterministic keys to use
    unsigned char sec[crypto_core_ristretto255_BYTES];
    key(sec, 1502);

    time_t now = time(0);

    // We can generate an entire list of things.
    printf("Random tokens:%ju\n", now);
    unsigned char A[num_tokens][crypto_core_ristretto255_BYTES];
	unsigned char tmp[crypto_core_ristretto255_SCALARBYTES];
    int i;
    int j;
    unsigned char arr[num_tokens];
    for (i=0; i<num_tokens; i++) {
        arr[0] = i;
        unsigned char H[64];
        crypto_generichash(H, 64, arr, 1, NULL, 0);
        crypto_core_ristretto255_from_hash(A[i], H);
    }
    for (i=0; i<num_print; i++) {
        print_ristretto(A[i], 32);
    }

    printf("\n");

    now = time(0);
    printf("Encrypted tokens:%ju\n", now);
    // Now let's encrypt the tokens by taking them to a secret exponent
    // Happens client side
    unsigned char hA[num_tokens][crypto_core_ristretto255_BYTES];
    exponentiate(hA, A, sec, num_tokens);
    //shuffle(hA, A, num_tokens);
    for (i=0; i<num_print; i++) {
        print_ristretto(hA[i], 32);
    }

    // Here we start the server's side

    printf("\n");

    now = time(0);
    printf("Shuffled and blinded tokens:%ju\n", now);
    // Let's now try shuffling and blinding
	unsigned char k[crypto_core_ristretto255_SCALARBYTES];
    //crypto_core_ristretto255_scalar_random(k);
    key(k, 501);
    unsigned char B[num_tokens][crypto_core_ristretto255_BYTES];
    shuffle_and_blind(B, hA, k, num_tokens);
    //exponentiate(B, hA, k, num_tokens);
    shuffle(B, hA, num_tokens);
    for (i=0; i<num_print; i++) {
        print_ristretto(B[i], 32);
    }
	
    printf("\n");

    now = time(0);
    printf("Unencrypted tokens:%ju\n", now);
    // Now let's try unencrypting the tokens
    // Happens client side
    unsigned char uB[num_tokens][crypto_core_ristretto255_BYTES];
    unsigned char dec[crypto_core_ristretto255_BYTES];
    crypto_core_ristretto255_scalar_invert(dec, sec);
    
    exponentiate(uB, B, dec, num_tokens);
    for (i=0; i<num_print; i++) {
        print_ristretto(uB[i], 32);
    }



    return 0;
}
