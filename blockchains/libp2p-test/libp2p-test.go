/*
	Copyright 2019 Whiteblock Inc.
	This file is a part of the genesis.

	Genesis is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    Genesis is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

//Package prysm handles prysm specific functionality
package libp2pTest

import (
	"encoding/json"
	"fmt"
	"github.com/Whiteblock/genesis/blockchains/helpers"
	"github.com/Whiteblock/genesis/blockchains/registrar"
	"github.com/Whiteblock/genesis/ssh"
	"github.com/Whiteblock/genesis/testnet"
	"github.com/Whiteblock/genesis/util"
	"regexp"
	"sync"
)

var conf *util.Config

const blockchain = "libp2p-test"

func init() {
	conf = util.GetConfig()

	registrar.RegisterBuild(blockchain, build)
	registrar.RegisterAddNodes(blockchain, add)
	registrar.RegisterServices(blockchain, func() []util.Service { return []util.Service{} })
	registrar.RegisterDefaults(blockchain, func() string { return "{}" })
	registrar.RegisterParams(blockchain, func() string { return "[]" })
}

type serialPeerInfo struct {
	ID     string   `json:"pid"`
	MAddrs []string `json:"addrs"`
}

// build builds out a fresh new prysm test network
func build(tn *testnet.TestNet) error {
	re := regexp.MustCompile(`(?m)(.*)Created a client(.*)`)
	peers := []serialPeerInfo{}
	mux := &sync.Mutex{}
	counter := 0
	interval := 1000000

	err := helpers.CreateConfigs(tn, "/p2p-tests/static-peers.json", func(node ssh.Node) ([]byte, error) {
		mux.Lock()
		defer mux.Unlock()
		out, err := json.Marshal(peers)
		if err != nil {
			return nil, util.LogError(err)
		}
		cmd := "/p2p-tests/p2p-tests --file /p2p-tests/static-peers.json"
		if counter == tn.LDD.Nodes-1 {
			cmd += fmt.Sprintf(" --send-interval %d", interval)
		}
		counter++
		_, err = tn.Clients[node.GetServerID()].DockerExecdit(node, fmt.Sprintf("bash -ic '%s 2>&1 | tee %s'", cmd, conf.DockerOutputFile))
		if err != nil {
			return nil, util.LogError(err)
		}

		for i := 0; i < 1000; i++ {
			res, err := tn.Clients[node.GetServerID()].DockerRead(node, conf.DockerOutputFile, -1)
			if err != nil {
				util.LogError(err)
				continue
			}
			matches := re.FindAllString(res, 1)
			if len(matches) > 0 {
				var peer serialPeerInfo
				err = json.Unmarshal([]byte(matches[0]), &peer)
				if err != nil {
					return nil, util.LogError(err)
				}
				peers = append(peers, peer)
				break
			}
		}
		fmt.Println("OUTPUT \n\n\n\n\n", string(out))
		return out, nil
	})
	if err != nil {
		return util.LogError(err)
	}
	return nil
}

// add handles adding nodes to the testnet
func add(tn *testnet.TestNet) error {
	return nil
}