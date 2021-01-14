// Package controller implements a controller for minogrpc.
//
// The controller can be used in a CLI to inject a dependency for Mino. It will
// start the overlay on the start command, and make sure resources are cleaned
// when the CLI daemon is stopping.
//
// Documentation Last Review: 07.10.2020
//
package controller

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"github.com/dedis/student20_rabyt/routing"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.dedis.ch/dela/serde"

	"go.dedis.ch/dela"
	"go.dedis.ch/dela/cli"
	"go.dedis.ch/dela/cli/node"
	"go.dedis.ch/dela/core/store/kv"
	"go.dedis.ch/dela/crypto/loader"
	"go.dedis.ch/dela/mino"
	"go.dedis.ch/dela/mino/minogrpc"
	"go.dedis.ch/dela/mino/minogrpc/certs"
	"go.dedis.ch/dela/mino/minogrpc/session"
	"golang.org/x/xerrors"
)

const certKeyName = "cert.key"

// MiniController is an initializer with the minimum set of commands.
//
// - implements node.Initializer
type miniController struct {
	random io.Reader
	curve  elliptic.Curve
}

// NewController returns a new initializer to start an instance of Minogrpc.
func NewController() node.Initializer {
	return miniController{
		random: rand.Reader,
		curve:  elliptic.P521(),
	}
}

// Build implements node.Initializer. It populates the builder with the commands
// to control Minogrpc.
func (m miniController) SetCommands(builder node.Builder) {
	builder.SetStartFlags(
		cli.IntFlag{
			Name:  "port",
			Usage: "set the port to listen on",
			Value: 2000,
		},
	)

	cmd := builder.SetCommand("minogrpc")
	cmd.SetDescription("Network overlay administration")

	sub := cmd.SetSubCommand("certificates")
	sub.SetDescription("list the certificates of the server")
	sub.SetAction(builder.MakeAction(certAction{}))

	sub = cmd.SetSubCommand("token")
	sub.SetDescription("generate a token to share to others to join the network")
	sub.SetFlags(
		cli.DurationFlag{
			Name:  "expiration",
			Usage: "amount of time before expiration",
			Value: 24 * time.Hour,
		},
	)
	sub.SetAction(builder.MakeAction(tokenAction{}))

	sub = cmd.SetSubCommand("join")
	sub.SetDescription("join a network of participants")
	sub.SetFlags(
		cli.StringFlag{
			Name:     "token",
			Usage:    "secret token generated by the node to join",
			Required: true,
		},
		cli.StringFlag{
			Name:     "address",
			Usage:    "address of the node to join",
			Required: true,
		},
		cli.StringFlag{
			Name:     "cert-hash",
			Usage:    "certificate hash of the distant server",
			Required: true,
		},
	)
	sub.SetAction(builder.MakeAction(joinAction{}))

	sub = cmd.SetSubCommand("stream")
	sub.SetDescription("stream a message to a list of addresses")
	sub.SetFlags(
		cli.StringSliceFlag{
			Name:     "addresses",
			Usage:    "list of addresses to send the message to",
			Required: true,
		},
		cli.StringFlag{
			Name:     "message",
			Usage:    "message to send",
			Required: true,
		},
	)
	sub.SetAction(builder.MakeAction(streamAction{addrsToSenderRecevier: make(map[string]senderReceiver)}))
}

// OnStart implements node.Initializer. It starts the minogrpc instance and
// injects it in the dependency resolver.
func (m miniController) OnStart(ctx cli.Flags, inj node.Injector) error {

	port := ctx.Int("port")
	if port < 0 || port > math.MaxUint16 {
		return xerrors.Errorf("invalid port value %d", port)
	}

	rter := routing.NewRouter(minogrpc.NewAddressFactory())

	myIP, err := findIP()
	if err != nil {
		return xerrors.Errorf("error finding ip: %v", err)
	}
	addr := minogrpc.ParseAddress(myIP, uint16(port))

	var db kv.DB
	err = inj.Resolve(&db)
	if err != nil {
		return xerrors.Errorf("injector: %v", err)
	}

	certs := certs.NewDiskStore(db, session.AddressFactory{})

	key, err := m.getKey(ctx)
	if err != nil {
		return xerrors.Errorf("cert private key: %v", err)
	}

	opts := []minogrpc.Option{
		minogrpc.WithStorage(certs),
		minogrpc.WithCertificateKey(key, key.Public()),
	}

	o, err := minogrpc.NewMinogrpc(addr, rter, opts...)
	if err != nil {
		return xerrors.Errorf("couldn't make overlay: %v", err)
	}

	inj.Inject(o)

	rpc := mino.MustCreateRPC(o, "test", exampleHandler{o.GetAddress()}, exampleFactory{})

	inj.Inject(rpc)

	dela.Logger.Info().Msgf("%v is running", o)

	return nil
}

// StoppableMino is an extension of Mino to allow one to stop the instance.
type StoppableMino interface {
	mino.Mino

	GracefulStop() error
}

// OnStop implements node.Initializer. It stops the network overlay.
func (m miniController) OnStop(inj node.Injector) error {
	var o StoppableMino
	err := inj.Resolve(&o)
	if err != nil {
		return xerrors.Errorf("injector: %v", err)
	}

	err = o.GracefulStop()
	if err != nil {
		return xerrors.Errorf("while stopping mino: %v", err)
	}

	return nil
}

func (m miniController) getKey(flags cli.Flags) (*ecdsa.PrivateKey, error) {
	loader := loader.NewFileLoader(filepath.Join(flags.Path("config"), certKeyName))

	keydata, err := loader.LoadOrCreate(newGenerator(m.random, m.curve))
	if err != nil {
		return nil, xerrors.Errorf("while loading: %v", err)
	}

	key, err := x509.ParseECPrivateKey(keydata)
	if err != nil {
		return nil, xerrors.Errorf("while parsing: %v", err)
	}

	return key, nil
}

// generator can generate a private key compatible with the x509 certificate.
//
// - implements loader.Generator
type generator struct {
	random io.Reader
	curve  elliptic.Curve
}

func newGenerator(r io.Reader, c elliptic.Curve) loader.Generator {
	return generator{
		random: r,
		curve:  c,
	}
}

// Generate implements loader.Generator. It returns the serialized data of a
// private key generated from the an elliptic curve. The data is formatted as a
// PEM block "EC PRIVATE KEY".
func (g generator) Generate() ([]byte, error) {
	priv, err := ecdsa.GenerateKey(g.curve, g.random)
	if err != nil {
		return nil, xerrors.Errorf("ecdsa: %v", err)
	}

	data, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, xerrors.Errorf("while marshaling: %v", err)
	}

	return data, nil
}

// exampleHandler is an RPC handler example.
//
// - implements mino.Handler
type exampleHandler struct {
	thisAddress mino.Address
}

// Process implements mino.Handler. It returns the message received.
func (exampleHandler) Process(req mino.Request) (serde.Message, error) {
	return req.Message, nil
}

// Message format ([part] -- optional part):
// msg#[Wait][NoReply][ReplyAll:addr1,addr2,...]
// Only one of NoReply and ReplyAll can be specified. The default behaviour is
// replying the sender
const (
	WaitCommand = "Wait"
	NoReplyCommand = "NoReply"
	ReplyAllCommand = "ReplyAll:"
	TextCommandSeparator = "#"
	AddressSeparator = ","
)

func (h exampleHandler) send(sender mino.Sender, reply string, addr string) {
	currErr := <-sender.Send(exampleMessage{reply}, session.NewAddress(addr))
	dela.Logger.Warn().Err(currErr).Msgf(
		"error when sending a message {%s} to %s", reply, addr)
}

func (h exampleHandler) replyMultiple(msgFormat string, addrs string,
	sender mino.Sender) {
	for _, addr := range strings.Split(addrs, AddressSeparator) {
		toAddr := session.NewAddress(addr)
		// do not send the message to self
		if toAddr.Equal(h.thisAddress) {
			continue
		}
		reply := fmt.Sprintf(msgFormat, addr)
		go h.send(sender, reply, addr)
	}
}

func (h exampleHandler) processMessage(from mino.Address, msg serde.Message,
	sender mino.Sender)  {

	commandText := msg.(exampleMessage).value
	parts := strings.Split(commandText, TextCommandSeparator)
	text := parts[0]
	command := ""
	if len(parts) > 1 {
		command = parts[1]
	}

	if strings.Contains(command, NoReplyCommand) {
		return
	}

	// Wait for the simulation to disconnect some links, and the disconnected
	// links to be discovered by grpc
	if strings.Contains(command, WaitCommand) {
		time.Sleep(30 * time.Second)
	}

	// the message to all participants has to include NoReply
	// to avoid infinite message exchange
	replyFormat := fmt.Sprintf(
		"%s's reply to %s for ", h.thisAddress, text) + "%s" +
		TextCommandSeparator + NoReplyCommand
	if strings.Contains(command, ReplyAllCommand) {
		addrStart := strings.Index(command, ReplyAllCommand) +
			len(ReplyAllCommand)
		h.replyMultiple(replyFormat, command[addrStart:], sender)
	}

	// Reply the sender (the sender either includes NoReply or is the
	// orchestrator, not included in the address list in ReplyAll)
	// Sleep before the reply to allow (potential) replyAll messages to arrive
	time.Sleep(time.Second)
	reply := fmt.Sprintf(replyFormat, from)
	err := <-sender.Send(exampleMessage{reply}, from)
	if err != nil {
		dela.Logger.Warn().Err(err).Msgf(
			"error when sending a message {%s} to orchestrator %s", reply, from)
	}
}

// Stream implements mino.Handler. It returns the message to the sender.
func (h exampleHandler) Stream(sender mino.Sender, recv mino.Receiver) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		from, msg, err := recv.Recv(ctx)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		dela.Logger.Info().Msgf("%s got %s from %s", h.thisAddress, msg,
			from.String())
		go h.processMessage(from, msg, sender)
	}
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

func findIP() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}

	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return "", err
	}

	dela.Logger.Info().Msgf("addrs: %v", addrs)
	return addrs[0], nil
}
