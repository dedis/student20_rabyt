package routing

import (
	"context"
	"fmt"
	"time"

	"go.dedis.ch/dela/mino"
	"go.dedis.ch/dela/mino/minogrpc"
	"go.dedis.ch/dela/serde"
)

func StreamN(numFollowers int, basePort uint16) {
	orchestrator, err := minogrpc.NewMinogrpc(minogrpc.ParseAddress(
		"127.0.0.1", basePort), NewRouter(minogrpc.NewAddressFactory()))
	if err != nil {
		panic("orchestrator overlay failed: " + err.Error())
	}
	orchestratorRPC := mino.MustCreateRPC(orchestrator, "test",
		exampleHandler{}, exampleFactory{})


	players := make([]*minogrpc.Minogrpc, numFollowers + 1)
	players[0] = orchestrator
	addresses := make([]mino.Address, numFollowers + 1)
	addresses[0] = orchestrator.GetAddress()
	addrToIdx := make(map[string]int)
	for i := 1; i <= numFollowers; i++ {
		player, err := minogrpc.NewMinogrpc(minogrpc.ParseAddress(
			"127.0.0.1", basePort + uint16(i)),
			NewRouter(minogrpc.NewAddressFactory()))
		if err != nil {
			panic("overlay " + string(rune(i)) + " failed: " + err.Error())
		}
		players[i] = player
		addresses[i] = player.GetAddress()
		addrToIdx[player.GetAddress().String()] = i
		mino.MustCreateRPC(player, "test", exampleHandler{}, exampleFactory{})
	}

	// set up certificates
	for i, firstPlayer := range players {
		for j, secondPlayer := range players {
			if i != j {
				firstPlayer.GetCertificateStore().Store(
					secondPlayer.GetAddress(), secondPlayer.GetCertificate())
			}
		}
	}

	addrs := mino.NewAddresses(addresses...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sender, recv, err := orchestratorRPC.Stream(ctx, addrs)
	if err != nil {
		panic("stream failed: " + err.Error())
	}

	msgFormatString := "Hello %d!"
	for i := 1; i <= numFollowers; i++ {
		msg := fmt.Sprintf(msgFormatString, i)
		err = <-sender.Send(exampleMessage{value: msg}, addresses[i])
		if err != nil {
			errorStr := fmt.Sprintf("failed to send to %d (%s): %s",
				i, addresses[i].String(), err.Error())
			panic(errorStr)
		}
	}

	for i := 0; i < numFollowers; i++ {
		from, msg, err := recv.Recv(ctx)
		if err != nil {
			panic("failed to receive: " + err.Error())
		}
		idx, ok := addrToIdx[from.String()]
		if !ok {
			panic("received a message from unexpected address: " + from.String())
		}
		expectedMsg := fmt.Sprintf(msgFormatString, idx)
		receivedMsg := msg.(exampleMessage).value
		if receivedMsg != expectedMsg {
			panic("expected " + expectedMsg + ", received " + receivedMsg)
		}
	}
	fmt.Println("Success")
}

func Example_stream_one() {
	StreamN(1, 2000)
	// Output: Success
}

func Example_stream_several() {
	StreamN(5, 3000)
	// Output: Success
}

func Example_stream_many() {
	StreamN(50, 4000)
	// Output: Success
}

// exampleHandler is an RPC handler example.
//
// - implements mino.Handler
type exampleHandler struct {
	mino.UnsupportedHandler
}

// Process implements mino.Handler. It returns the message received.
func (exampleHandler) Process(req mino.Request) (serde.Message, error) {
	return req.Message, nil
}

// Stream implements mino.Handler. It returns the message to the sender.
func (exampleHandler) Stream(sender mino.Sender, recv mino.Receiver) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	from, msg, err := recv.Recv(ctx)
	if err != nil {
		return err
	}

	err = <-sender.Send(msg, from)
	if err != nil {
		return err
	}

	return nil
}

// exampleMessage is an example of a message.
//
// - implements serde.Message
type exampleMessage struct {
	value string
}

// Serialize implements serde.Message. It returns the value contained in the
// message.
func (m exampleMessage) Serialize(serde.Context) ([]byte, error) {
	return []byte(m.value), nil
}

// exampleFactory is an example of a factory.
//
// - implements serde.Factory
type exampleFactory struct{}

// Deserialize implements serde.Factory. It returns the message using data as
// the inner value.
func (exampleFactory) Deserialize(ctx serde.Context, data []byte) (serde.Message, error) {
	return exampleMessage{value: string(data)}, nil
}
