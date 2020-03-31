IDIR=.
CC=gcc
#LIB=/home/ywy/bin/lib
#CFLAGS=-I${IDIR} -I/home/ywy/bin/include -std=c++11 -pipe -L${LIB} -Wall -O3 -DNDEBUG -fopenmp \
# -pg \
	   #-ftree-vectorizer-verbose=2
CFLAGS=-I${IDIR} -lsodium

OBJS = $(patsubst %.c,%.o,$(wildcard *.c))

all: ${OBJS} main
	echo "All made."

main:
	${CC} -o $@ $@.o ${CFLAGS}

%.o: %.c
	${CC} ${CFLAGS} -c -o $@ $<

clean:
	rm -f ${OBJS} main
	@echo "All cleaned up!"
