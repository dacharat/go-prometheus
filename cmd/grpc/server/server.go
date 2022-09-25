package server

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	hellov1 "github.com/dacharat/go-prometheus/internal/gen/proto/hello/v1"
)

type helloHandler struct {
	hellov1.UnimplementedHelloServiceServer
}

func NewHelloHandler() hellov1.HelloServiceServer {
	return &helloHandler{}
}

func (h helloHandler) GetGreeting(ctx context.Context, req *hellov1.GetGreetingRequest) (*hellov1.GetGreetingResponse, error) {
	nBig, err := rand.Int(rand.Reader, big.NewInt(1200))
	if err != nil {
		return nil, err
	}

	n := nBig.Int64()
	if n > 1100 {
		return nil, errors.New("random more than 1000")
	}

	t := time.Duration(n)
	time.Sleep(time.Millisecond * t)

	return &hellov1.GetGreetingResponse{
		Message: "hi",
	}, nil
}
