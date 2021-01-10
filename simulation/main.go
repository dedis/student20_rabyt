package main

import (
	"bytes"
	"flag"
	"fmt"
	"go.dedis.ch/simnet/sim/kubernetes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.dedis.ch/simnet"
	"go.dedis.ch/simnet/network"
	"go.dedis.ch/simnet/sim"
	"golang.org/x/xerrors"
)

type simRound struct{}

func (s simRound) Before(simio sim.IO, nodes []sim.NodeInfo) error {
	reader, writer := io.Pipe()

	go io.Copy(os.Stdout, reader)

	// Exchange certs
	buf := new(bytes.Buffer)
	err := simio.Exec(nodes[0].Name, []string{"./memcoin", "--config", "/tmp/node", "minogrpc", "token"}, sim.ExecOptions{
		Stdout: buf,
	})
	if err != nil {
		return xerrors.Errorf("error getting token: %v", err)
	}
	token := strings.Split(buf.String(), " ")
	tokenCmd := []string{
				"./memcoin", "--config", "/tmp/node", "minogrpc", "join",
				"--address", fmt.Sprintf("%s:2000", nodes[0].Address)}
	tokenCmd = append(tokenCmd, token...)
	for i := 1; i < len(nodes); i++ {
		err := simio.Exec(
			nodes[i].Name,
			tokenCmd,
			sim.ExecOptions{
				Stdout: writer,
				Stderr: writer,
			},
		)
		if err != nil {
			return err
		}
	}

	// Exchange messages
	cmd := []string{"./memcoin", "--config", "/tmp/node", "minogrpc", "stream"}
	for i := 1; i < len(nodes); i++ {
		cmd = append(cmd, []string{"--addresses", fmt.Sprintf("F%s:2000", nodes[i].Address)}...)
	}
	cmd = append(cmd, []string{"--message", "SetupMessage"}...)
	err = simio.Exec(nodes[0].Name, cmd, sim.ExecOptions{
		Stdout: writer,
		Stderr: writer,
	})

	return nil
}

func (s simRound) Execute(simio sim.IO, nodes []sim.NodeInfo) error {
	fmt.Printf("Orchestrator is: %s at %s\n", nodes[0].Name, nodes[0].Address)
	reader, writer := io.Pipe()

	go io.Copy(os.Stdout, reader)

	// Exchange messages
	cmd := []string{"./memcoin", "--config", "/tmp/node", "minogrpc", "stream"}
	for i := 1; i < len(nodes); i++ {
		cmd = append(cmd, []string{"--addresses", fmt.Sprintf("F%s:2000", nodes[i].Address)}...)
	}
	cmd = append(cmd, []string{"--message", "TrueMessage"}...)

	err := simio.Exec(nodes[0].Name, cmd, sim.ExecOptions{
		Stdout: writer,
		Stderr: writer,
	})
	if err != nil {
		return err
	}

	writer.Close()
	return nil
}

func (s simRound) After(simio sim.IO, nodes []sim.NodeInfo) error {
	return nil
}

func createSimOptions(numNodes int, dockerImage string) []sim.Option {
	return []sim.Option{
		sim.WithTopology(
			network.NewSimpleTopology(numNodes, 20*time.Millisecond),
		),
		sim.WithImage(dockerImage, nil, nil,
			sim.NewTCP(2000)),
		kubernetes.WithResources("20m", "64Mi"),
	}
}

const (
	TreeRoutingImage   = "katjag/dela-tree-simulation"
	PrefixRoutingImage = "katjag/prefix-routing-simulation"
)

func runSimulation(numNodes int, dockerImage string) error {
	options := createSimOptions(numNodes, dockerImage)

	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	engine, err := kubernetes.NewStrategy(kubeconfig, options...)
	if err != nil {
		return err
	}

	simulation := simnet.NewSimulation(simRound{}, engine)

	// os.Args might include arguments for this simulation as well as arguments
	// for simnet. Look for --, separating the two argument groups
	simnetExecName := fmt.Sprintf("%s : simnet", os.Args[0])
	for i, arg := range os.Args {
		if arg == "--" {
			simArgs := make([]string, len(os.Args)-i)
			copy(simArgs, os.Args[i:])
			// will be used as the executable name
			simArgs[0] = simnetExecName
			err = simulation.Run(simArgs)
			return err
		}
	}
	// "--" not found, therefore arguments for simnet are not provided
	err = simulation.Run([]string{simnetExecName})
	return err
}

func main() {
	var numNodes int
	var routingProtocol string
	algoToImage := map[string]string{"tree" : TreeRoutingImage, "prefix": PrefixRoutingImage}

	flag.IntVar(&numNodes, "n", 10, "the number of nodes for simulation")
	flag.StringVar(&routingProtocol, "protocol", "prefix",
		"the routing protocol: must be 'tree' or 'prefix'")

	flag.Parse()

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr,
			"Usage of routing protocol simulation: "+
				"simulation-options -- simnet-options")
		fmt.Fprintln(os.Stderr, "Simulation options are:")
		flag.PrintDefaults()
	}

	image, ok := algoToImage[routingProtocol]
	if !ok {
		panic(fmt.Errorf("unexpected routing protocol requested: %s. " +
			"Allowed --protocol values: %s", routingProtocol, algoToImage))
	}

	err := runSimulation(numNodes, image)
	if err != nil {
		panic(err)
	}
}
