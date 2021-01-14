package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go.dedis.ch/simnet/sim/kubernetes"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.dedis.ch/simnet"
	"go.dedis.ch/simnet/network"
	"go.dedis.ch/simnet/sim"
	"golang.org/x/xerrors"
)

func init() {
	s := time.Now().UnixNano()
	rand.Seed(s)
}

type simRound struct {
	noCerts                         bool
	replyAll                        bool
	disconnectBeforeOrchestratorMsg bool
	disconnectAfterOrchestratorMsg  bool
	// percentage of broken links
	disconnectPercentage int
	dropUsedLinks        bool

	logsDir string
}

type nodePair struct {
	first  sim.NodeInfo
	second sim.NodeInfo
}

func getToken(simio sim.IO, node sim.NodeInfo) ([]string, error) {
	buf := new(bytes.Buffer)
	cmd := []string{"./memcoin", "--config", "/tmp/node", "minogrpc", "token"}
	err := simio.Exec(node.Name, cmd, sim.ExecOptions{
		Stdout: buf,
	})
	if err != nil {
		return nil, xerrors.Errorf("error getting token: %v", err)
	}
	token := strings.Split(buf.String(), " ")
	return token, nil
}

func executeJoin(simio sim.IO, joiningNode sim.NodeInfo,
	tokenNode sim.NodeInfo, token []string, writer io.Writer) error {
	tokenCmd := []string{
		"./memcoin", "--config", "/tmp/node", "minogrpc", "join",
		"--address", fmt.Sprintf("%s:2000", tokenNode.Address)}
	tokenCmd = append(tokenCmd, token...)
	return simio.Exec(
		joiningNode.Name,
		tokenCmd,
		sim.ExecOptions{
			Stdout: writer,
			Stderr: writer,
		},
	)
}

func sendToken(simio sim.IO, joiningNode sim.NodeInfo,
	tokenNodes []sim.NodeInfo, tokens [][]string, writer io.Writer,
	wg *sync.WaitGroup, failed chan nodePair) {
	defer wg.Done()

	for i := 0; i < len(tokenNodes); i++ {
		err := executeJoin(simio, joiningNode, tokenNodes[i], tokens[i], writer)
		if err != nil {
			writer.Write([]byte(fmt.Sprintf("error joining: %v\n", err)))
			failed <- nodePair{tokenNodes[i], joiningNode}
		}
	}
}

type Link struct {
	src  string
	dest string
}

func addLink(src string, dest string, srcToDest map[string][]string) {
	dests, in := srcToDest[src]
	if in {
		srcToDest[src] = append(dests, dest)
	} else {
		srcToDest[src] = []string{dest}
	}
}

func disconnectLinks(simio sim.IO, links []Link,
	disconnectPercentage int) error {
	numToDisconnect := disconnectPercentage * len(links) / 100
	// +1 to ceil()
	if (disconnectPercentage*len(links))%100 > 0 {
		numToDisconnect += 1
	}
	fmt.Printf("Disconnecting %d out of %d links (~%d%%)\n", numToDisconnect,
		len(links), disconnectPercentage)
	// group links by source
	linksToDisconnect := make(map[string][]string)
	// choose links to disconnect at random by shuffling links and
	// taking first numToDisconnect links
	// shuffling is done implicitly by generating a random permutation of
	// indices (to keep the links slice unmodified)
	indexPermutation := rand.Perm(len(links))
	for i := 0; i < numToDisconnect; i++ {
		toDisconnect := links[indexPermutation[i]]
		src, dest := toDisconnect.src, toDisconnect.dest
		// Disconnect only disconnects links one way, so add both directions
		// to break the link
		addLink(src, dest, linksToDisconnect)
		addLink(dest, src, linksToDisconnect)
	}
	for src, dests := range linksToDisconnect {
		err := simio.Disconnect(src, dests...)
		if err != nil {
			fmt.Printf("error disconnecting: %s\n", err.Error())
			return err
		}
		fmt.Printf("disconnected %s <-> %s\n", src, dests)
	}
	return nil
}

func (s simRound) candidatesToDisconnect(nodes []sim.NodeInfo) ([]Link, error) {
	if s.dropUsedLinks {
		// TODO: a more reliable way
		output, err := exec.Command("./calculate_statistics.py",
			"--hops").Output()
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		hops := strings.Split(string(output), "\n")
		links := make([]Link, 0, len(hops))
		for _, hop := range hops {
			srcDest := strings.Split(hop, " ")
			if len(srcDest) < 2 {
				continue
			}
			src := srcDest[0]
			dest := srcDest[1]
			links = append(links, Link{src, dest})
		}
		return links, nil
	}
	links := make([]Link, 0, len(nodes)*(len(nodes)-1))
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			links = append(links, Link{nodes[i].Name, nodes[j].Name})
		}
	}
	return links, nil
}

func retryFailed(simio sim.IO, failed chan nodePair, writer io.Writer,
	wg *sync.WaitGroup, errs chan error) {
	defer wg.Done()
	defer close(errs)

	failures := make([]nodePair, 0)
	for np := range failed {
		failures = append(failures, np)
	}
	for _, np := range failures {
		token, err := getToken(simio, np.first)
		if err != nil {
			errs <- err
		}
		err = executeJoin(simio, np.second, np.first, token, writer)
		if err != nil {
			errs <- err
		}
	}
}

func (s simRound) Before(simio sim.IO, nodes []sim.NodeInfo) error {
	return nil
}

func (s simRound) createMessage(text string, destinations []sim.NodeInfo) string {
	var builder strings.Builder
	builder.WriteString(text)
	builder.WriteString("#")
	if s.disconnectAfterOrchestratorMsg {
		// wait before replying so that simnet has enough time to break links
		builder.WriteString("Wait")
	}
	if s.replyAll {
		builder.WriteString("ReplyAll:")
		for i, n := range destinations {
			builder.WriteString(fmt.Sprintf("%s:2000", n.Address))
			// no trailing comma
			if i < len(destinations)-1 {
				builder.WriteString(",")
			}
		}
	}
	return builder.String()
}

func createSendMessageCommand(text string,
	destinations []sim.NodeInfo) []string {
	cmd := []string{"./memcoin", "--config", "/tmp/node", "minogrpc", "stream"}
	for _, n := range destinations {
		cmd = append(cmd, []string{"--addresses", fmt.Sprintf("F%s:2000",
			n.Address)}...)
	}
	return append(cmd, []string{"--message", text}...)
}

func checkMessageDelivered(wg *sync.WaitGroup, file *os.File, msg string) {
	defer wg.Done()

	for {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			text := scanner.Text()
			if strings.Contains(text, "got") && strings.Contains(text, msg) {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func sendMessage(simio sim.IO, node sim.NodeInfo, cmd []string, ready chan struct{}) {
	defer close(ready)
	reader, writer := io.Pipe()
	defer writer.Close()
	go io.Copy(os.Stdout, reader)

	fmt.Println("orchestrator broadcasts the message and waits for replies")
	simio.Exec(node.Name, cmd, sim.ExecOptions{
		Stdout: writer,
		Stderr: writer,
	})
	fmt.Println("orchestrator done")
}

func (s simRound) exchangeCertificates(simio sim.IO,
	nodes []sim.NodeInfo) error {
	reader, writer := io.Pipe()
	go io.Copy(os.Stdout, reader)

	// Exchange certs
	tokens := make([][]string, len(nodes), len(nodes))
	for i := 0; i < len(nodes); i++ {
		token, err := getToken(simio, nodes[i])
		if err != nil {
			return err
		}
		tokens[i] = token
	}
	failedExchangeChannel := make(chan nodePair)
	retryErrors := make(chan error)
	var waitRetry sync.WaitGroup
	waitRetry.Add(1)
	go retryFailed(simio, failedExchangeChannel, writer, &waitRetry,
		retryErrors)

	var wg sync.WaitGroup
	for i := 0; i < len(nodes); i++ {
		// The connection with previous nodes is already established
		wg.Add(1)
		go sendToken(simio, nodes[i], nodes[i+1:], tokens[i+1:], writer,
			&wg, failedExchangeChannel)
		// all nodes joined the first node and it's enough for broadcast
		if !s.replyAll {
			break
		}
	}
	wg.Wait()
	close(failedExchangeChannel)
	waitRetry.Wait()

	doubleFailures := 0
	for err := range retryErrors {
		fmt.Printf("Certificate exchange failed twice: %v\n", err)
		doubleFailures++
	}
	if doubleFailures > 0 {
		return fmt.Errorf("certificate exchange failed twice %d times, "+
			"exiting", doubleFailures)
	}

	return nil
}

func (s simRound) Execute(simio sim.IO, nodes []sim.NodeInfo) error {
	if s.noCerts {
		fmt.Println("skipping certificate exchange")
	} else {
		err := s.exchangeCertificates(simio, nodes)
		if err != nil {
			return err
		}
		fmt.Println("finished certificate exchange")
	}

	if s.disconnectBeforeOrchestratorMsg {
		links, err := s.candidatesToDisconnect(nodes)
		if err != nil {
			return err
		}
		err = disconnectLinks(simio, links, s.disconnectPercentage)
		if err != nil {
			return err
		}
		fmt.Println("disconnected links")
	}

	// Exchange messages. Destinations are all nodes but orchestrator
	msgWithCommands := s.createMessage("Message", nodes[1:])
	cmd := createSendMessageCommand(msgWithCommands, nodes[1:])
	ready := make(chan struct{})
	// TODO: can I use simio concurrently?
	go sendMessage(simio, nodes[0], cmd, ready)

	if s.disconnectAfterOrchestratorMsg {
		// wait until the message from the orchestrator is delivered to all
		// nodes
		logFiles, err := ioutil.ReadDir(s.logsDir)
		if err != nil {
			return fmt.Errorf("error reading log directory: %w", err)
		}
		var wg sync.WaitGroup
		for _, log := range logFiles {
			// skip orchestrator's log
			if strings.Contains(log.Name(), "-"+nodes[0].Name+"-") {
				continue
			}
			file, err := os.Open(filepath.Join(s.logsDir, log.Name()))
			if err != nil {
				return fmt.Errorf("error reading a log file %s: %w",
					log.Name(), err)
			}
			wg.Add(1)
			go checkMessageDelivered(&wg, file, msgWithCommands)
		}
		fmt.Println("waiting for broadcast message to arrive to followers")
		wg.Wait()

		links, err := s.candidatesToDisconnect(nodes)
		if err != nil {
			return err
		}
		err = disconnectLinks(simio, links, s.disconnectPercentage)
		if err != nil {
			return err
		}
	}

	<-ready
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
		kubernetes.WithResources("20m", "20Mi"),
	}
}

const (
	TreeRoutingImage   = "katjag/dela-tree-simulation"
	PrefixRoutingImage = "katjag/prefix-routing-simulation"
	NaiveRoutingImage  = "katjag/naive-prefix-routing-simulation"
)

func runSimulation(numNodes int, dockerImage string, round simRound) error {
	options := createSimOptions(numNodes, dockerImage)

	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	engine, err := kubernetes.NewStrategy(kubeconfig, options...)
	if err != nil {
		return err
	}

	// Save the logs directory location in the simulation round
	round.logsDir = filepath.Join(sim.NewOptions(options).OutputDir, "logs")
	simulation := simnet.NewSimulation(round, engine)

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

const (
	noCertsFlag          = "no-certs"
	numNodesFlag         = "n"
	protocolFlag         = "protocol"
	replyAllFlag         = "replyAll"
	disconnectBeforeFlag = "disconnect-before"
	disconnectAfterFlag  = "disconnect-after"
	percentageFlag       = "disconnect-percentage"
	usedLinksFlag        = "drop-used-links"
)

func main() {
	var numNodes int
	var routingProtocol string
	var s simRound
	algoToImage := map[string]string{"tree": TreeRoutingImage,
		"prefix": PrefixRoutingImage, "naive": NaiveRoutingImage}

	flag.BoolVar(&s.noCerts, noCertsFlag, true, "skip certificate exchange")
	flag.IntVar(&numNodes, numNodesFlag, 10, "the number of nodes for simulation")
	flag.StringVar(&routingProtocol, protocolFlag, "prefix",
		"the routing protocol: must be 'tree' or 'prefix'")
	flag.BoolVar(&s.replyAll, replyAllFlag, false,
		"upon receiving the message from orchestrator, "+
			"followers send a message to all other participants")
	flag.BoolVar(&s.disconnectBeforeOrchestratorMsg, disconnectBeforeFlag, false,
		"break some network links before any messages are sent."+
			fmt.Sprintf("See --%s for further options", percentageFlag))
	flag.BoolVar(&s.disconnectAfterOrchestratorMsg, disconnectAfterFlag, false,
		"break some network links after orchestrator's message is sent but "+
			"before the replies."+
			fmt.Sprintf("See --%s and --%s for further options",
				percentageFlag, usedLinksFlag))
	flag.IntVar(&s.disconnectPercentage, percentageFlag, 10,
		"percentage of broken links."+
			fmt.Sprintf("Has effect only if --%s or --%s is set",
				disconnectBeforeFlag, disconnectAfterFlag))
	flag.BoolVar(&s.dropUsedLinks, "drop-used-links", false,
		"drop links, used for routing (as opposed to a random subset)."+
			fmt.Sprintf("Has effect only if --%s is set", disconnectAfterFlag))

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
		panic(fmt.Errorf("unexpected routing protocol requested: %s. "+
			"Allowed --%s values: %s", routingProtocol, protocolFlag,
			algoToImage))
	}
	if s.disconnectBeforeOrchestratorMsg && s.disconnectAfterOrchestratorMsg {
		panic(fmt.Errorf("--%s and --%s cannot be both set",
			disconnectBeforeFlag, disconnectAfterFlag))
	}
	if (s.disconnectBeforeOrchestratorMsg || s.disconnectAfterOrchestratorMsg) && s.
		disconnectPercentage == 0 {
		panic(fmt.Errorf("network disconnect requested but %s = 0", percentageFlag))
	}
	if s.disconnectPercentage < 0 || s.disconnectPercentage > 100 {
		panic(fmt.Errorf("--%s values should be between 0 and 100. Got: %d",
			percentageFlag, s.disconnectPercentage))
	}
	if s.disconnectBeforeOrchestratorMsg && s.dropUsedLinks {
		panic(fmt.Errorf("--%s can only be specified with --%s, otherwise "+
			"used links are not known", usedLinksFlag, disconnectAfterFlag))
	}

	err := runSimulation(numNodes, image, s)
	if err != nil {
		panic(err)
	}
}
