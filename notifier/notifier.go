package notifier

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"github.com/yunwilliamyu/contact-trace-mixnet/notifier/pb"
	"google.golang.org/grpc"
	"io"
	"log"
)

//go:generate protoc pb/notifier.proto --go_out=plugins=grpc:.

const IDSize = 16

type DeadDropID [IDSize]byte

type DB interface {
	Put(deaddropID DeadDropID, message *pb.Notification) error
	Fetch(deaddropID DeadDropID, handler func(messages []*pb.Notification) (dropPrefix int, err error)) error
}

type Config struct {
	ServerAddr string
	// server endpoint for polling
	// separate endpoint for notifications?
	PublicKey [32]byte
}

func (c Config) DialService(ctx context.Context) (pb.NotifierClient, error) {
	cc, err := grpc.DialContext(ctx, c.ServerAddr)
	if err != nil {
		return nil, err
	}
	return pb.NewNotifierClient(cc), nil
}

type PollServer struct {
	pb.UnimplementedNotifierServer

	conf       *Config
	privateKey [32]byte
	db         DB
}

func (ps *PollServer) FetchNotifications(ctx context.Context, req *pb.FetchRequest) (*pb.FetchResponse, error) {
	resp := &pb.FetchResponse{}
	var deaddropID DeadDropID
	copy(deaddropID[:], req.GetDeaddropId()) // check len
	lastRead := req.GetLastRead()

	if err := ps.db.Fetch(deaddropID, func(messages []*pb.Notification) (dropPrefix int, err error) {
		startIdx := 0
		for i, msg := range messages {
			if len(lastRead) > 0 && bytes.Equal(lastRead, msg.Contents[:len(lastRead)]) {
				startIdx = i + 1
				break
			}
		}
		resp.Notifications = messages[startIdx:]
		return startIdx, nil
	}); err != nil {
		return nil, err
	}
	return resp, nil
}

func (ps *PollServer) PostNotificationV1(ctx context.Context, req *pb.PostRequestV1) (*pb.Empty, error) {
	hint, deaddropID, err := ps.unsealAddressV1(req.GetSealedAddress())
	if err != nil {
		return nil, err
	}
	n := &pb.Notification{
		Hint:     uint32(hint),
		Contents: req.Contents,
	}
	if err := ps.db.Put(deaddropID, n); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

type NotifierClient struct {
	conf *Config
	stub pb.NotifierClient
}

func NewNotifierClient(conf *Config) (*NotifierClient, error) {
	nc := &NotifierClient{conf: conf}
	var err error
	nc.stub, err = conf.DialService(context.TODO())
	if err != nil {
		return nil, err
	}
	return nc, nil
}

func (nc *NotifierClient) Notify(ctx context.Context, address []byte, msg []byte) error {
	log.Printf("Notification to %s", address)
	_, err := nc.stub.PostNotificationV1(ctx, &pb.PostRequestV1{
		SealedAddress: address,
		Contents:      msg,
	})
	return err
}

const LastReadLength = 2

func NewRandomDeadDropClient(conf *Config) (*DeadDropClient, error) {
	dcl := &DeadDropClient{conf: conf}
	var err error
	dcl.stub, err = conf.DialService(context.TODO())
	if err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(cryptorand.Reader, dcl.address[:]); err != nil {
		return nil, err
	}
	return dcl, nil
}

type DeadDropClient struct {
	conf    *Config
	stub    pb.NotifierClient
	address [IDSize]byte

	lastReceived [LastReadLength]byte
}

func (dcl *DeadDropClient) Poll(ctx context.Context) ([]*pb.Notification, error) {
	resp, err := dcl.stub.FetchNotifications(ctx, &pb.FetchRequest{
		DeaddropId: dcl.address[:],
		LastRead:   dcl.lastReceived[:],
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Notifications) > 0 {
		copy(dcl.lastReceived[:], resp.Notifications[len(resp.Notifications)-1].Contents)
	}
	return resp.Notifications, nil
}
