package netconf

import (
	db "../db"
	ssh "../ssh"
	status "../status"
	util "../util"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
)

type Connection struct {
	To   int `json:"to"`
	From int `json:"from"`
}

func RemoveAllOutages(client *ssh.Client) error {
	res, err := client.Run("sudo iptables --list-rules | grep wb_bridge | grep DROP | grep FORWARD || true")
	if err != nil {
		log.Println(err)
		return err
	}
	if len(res) == 0 {
		return nil
	}
	res = strings.Replace(res, "-A ", "", -1)
	cmds := strings.Split(res, "\n")
	wg := sync.WaitGroup{}

	for _, cmd := range cmds {
		if len(cmd) == 0 {
			continue
		}
		wg.Add(1)
		go func(cmd string) {
			defer wg.Done()
			_, err = client.Run(fmt.Sprintf("sudo iptables -D %s", cmd))
			if err != nil {
				log.Println(err)
			}
		}(cmd)
	}

	wg.Wait()
	return nil
}

func MakeOutageCommands(node1 db.Node, node2 db.Node) []string {
	return []string{
		fmt.Sprintf("FORWARD -i %s%d -d %s -j DROP", conf.BridgePrefix, node1.AbsoluteNum, node2.Ip),
		fmt.Sprintf("FORWARD -i %s%d -d %s -j DROP", conf.BridgePrefix, node2.AbsoluteNum, node1.Ip),
	}
}

func MakeOutage(node1 db.Node, node2 db.Node) error {
	cmds := MakeOutageCommands(node1, node2)

	client, err := status.GetClient(node1.Server)
	if err != nil {
		log.Println(err)
		return err
	}
	_, err = client.Run(fmt.Sprintf("sudo iptables -I %s", cmds[0]))
	if err != nil {
		log.Println(err)
		return err
	}
	client, err = status.GetClient(node2.Server)
	if err != nil {
		log.Println(err)
		return err
	}
	_, err = client.Run(fmt.Sprintf("sudo iptables -I %s", cmds[1]))
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func RemoveOutage(node1 db.Node, node2 db.Node) error {

	cmds := MakeOutageCommands(node1, node2)

	client, err := status.GetClient(node1.Server)
	if err != nil {
		log.Println(err)
		return err
	}
	_, err = client.Run(fmt.Sprintf("sudo iptables -D %s", cmds[0]))
	if err != nil {
		log.Println(err)
		return err
	}
	client, err = status.GetClient(node2.Server)
	if err != nil {
		log.Println(err)
		return err
	}
	_, err = client.Run(fmt.Sprintf("sudo iptables -D %s", cmds[1]))
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func CreatePartitionOutage(side1 []db.Node, side2 []db.Node) { //Doesn't report errors yet
	wg := sync.WaitGroup{}
	for _, node1 := range side1 {
		for _, node2 := range side2 {
			wg.Add(1)
			go func(node1 db.Node, node2 db.Node) {
				defer wg.Done()
				err := MakeOutage(node1, node2)
				if err != nil {
					log.Println(err)
				}
			}(node1, node2)
		}
	}
	wg.Wait()
}

//TODO: Naive Implementation, does not yet take multiple servers into account
func GetCutConnections(client *ssh.Client) ([]Connection, error) {
	res, err := client.Run("sudo iptables --list-rules | grep wb_bridge | grep DROP | grep FORWARD | awk '{print $4,$6}' | sed -e 's/\\/32//g' || true")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	out := []Connection{}
	if len(res) == 0 { //No cut connections on this server
		return out, nil
	}

	cuts := strings.Split(res, "\n")

	for _, cut := range cuts {
		if len(cut) == 0 {
			continue
		}
		cutPair := strings.Split(cut, " ")
		if len(cutPair) != 2 {
			return nil, fmt.Errorf("unexpected result \"%s\" for cut pair", cut)
		}
		_, toNode := util.GetInfoFromIP(cutPair[0])

		if len(cutPair[1]) <= len(conf.BridgePrefix) {
			return nil, fmt.Errorf("unexpected source interface, found \"%s\"", cutPair[1])
		}

		fromNode, err := strconv.Atoi(cutPair[1][len(conf.BridgePrefix):])
		if err != nil {
			log.Println(err)
			return nil, err
		}
		out = append(out, Connection{To: toNode, From: fromNode})
	}
	return out, nil
}
