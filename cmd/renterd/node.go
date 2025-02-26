package main

import (
	"log"
	"os"
	"path/filepath"

	"go.sia.tech/renterd/internal/consensus"
	"go.sia.tech/renterd/internal/stores"
	"go.sia.tech/renterd/wallet"
	"go.sia.tech/siad/modules"
	mconsensus "go.sia.tech/siad/modules/consensus"
	"go.sia.tech/siad/modules/gateway"
	"go.sia.tech/siad/modules/transactionpool"
)

type node struct {
	g   modules.Gateway
	cm  modules.ConsensusSet
	tp  modules.TransactionPool
	w   *wallet.SingleAddressWallet
	hdb *stores.JSONHostDB
	cs  *stores.JSONContractStore
	os  *stores.JSONObjectStore
}

func (n *node) Close() error {
	errs := []error{
		n.g.Close(),
		n.cm.Close(),
		n.tp.Close(),
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func newNode(addr, dir string, bootstrap bool, walletKey consensus.PrivateKey) (*node, error) {
	gatewayDir := filepath.Join(dir, "gateway")
	if err := os.MkdirAll(gatewayDir, 0700); err != nil {
		return nil, err
	}
	g, err := gateway.New(addr, bootstrap, gatewayDir)
	if err != nil {
		return nil, err
	}
	consensusDir := filepath.Join(dir, "consensus")
	if err := os.MkdirAll(consensusDir, 0700); err != nil {
		return nil, err
	}
	cm, errCh := mconsensus.New(g, bootstrap, consensusDir)
	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	default:
		go func() {
			if err := <-errCh; err != nil {
				log.Println("WARNING: consensus initialization returned an error:", err)
			}
		}()
	}
	tpoolDir := filepath.Join(dir, "transactionpool")
	if err := os.MkdirAll(tpoolDir, 0700); err != nil {
		return nil, err
	}
	tp, err := transactionpool.New(cm, g, tpoolDir)
	if err != nil {
		return nil, err
	}

	walletDir := filepath.Join(dir, "wallet")
	if err := os.MkdirAll(walletDir, 0700); err != nil {
		return nil, err
	}
	walletAddr := wallet.StandardAddress(walletKey.PublicKey())
	ws, ccid, err := stores.NewJSONWalletStore(walletDir, walletAddr)
	if err != nil {
		return nil, err
	} else if err := cm.ConsensusSetSubscribe(ws, ccid, nil); err != nil {
		return nil, err
	}
	w := wallet.NewSingleAddressWallet(walletKey, ws)

	hostdbDir := filepath.Join(dir, "hostdb")
	if err := os.MkdirAll(hostdbDir, 0700); err != nil {
		return nil, err
	}
	hdb, ccid, err := stores.NewJSONHostDB(hostdbDir)
	if err != nil {
		return nil, err
	} else if err := cm.ConsensusSetSubscribe(hdb, ccid, nil); err != nil {
		return nil, err
	}

	contractsDir := filepath.Join(dir, "contracts")
	if err := os.MkdirAll(contractsDir, 0700); err != nil {
		return nil, err
	}
	cs, err := stores.NewJSONContractStore(contractsDir)
	if err != nil {
		return nil, err
	}

	objectsDir := filepath.Join(dir, "objects")
	if err := os.MkdirAll(objectsDir, 0700); err != nil {
		return nil, err
	}
	os, err := stores.NewJSONObjectStore(objectsDir)
	if err != nil {
		return nil, err
	}

	return &node{
		g:   g,
		cm:  cm,
		tp:  tp,
		w:   w,
		hdb: hdb,
		cs:  cs,
		os:  os,
	}, nil
}
