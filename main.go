package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/julienschmidt/httprouter"
)

const t = "ws://localhost:8545"

var (
	clientDial = flag.String(
		"client_dial", t, "could be websocket or IPC",
	)
)

func toCallArg(msg ethereum.CallMsg) interface{} {
	arg := map[string]interface{}{
		"from": msg.From,
		"to":   msg.To,
	}

	if len(msg.Data) > 0 {
		arg["data"] = hexutil.Bytes(msg.Data)
	}
	if msg.Value != nil {
		arg["value"] = (*hexutil.Big)(msg.Value)
	}

	if msg.Gas != 0 {
		arg["gas"] = msg.Gas
	}

	if msg.GasPrice != nil {
		arg["gasPrice"] = (*hexutil.Big)(msg.GasPrice)
	}

	return arg
}

// tODO com eback to
func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, "Welcome!\n")
}

var (
	empty = common.Address{}
)

func (h *withRPCHandle) evalParams(
	w http.ResponseWriter, r *http.Request, ps httprouter.Params,
) {
	addr := common.HexToAddress(ps.ByName("addr"))
	if addr == empty {
	} else {
		params := strings.Split(ps.ByName("contractParams"), "/")
		if len(params) == 0 {
			// TODO some html
			return
		}

		methodSig := common.Hex2Bytes(ps.ByName("methodSig"))
		msg := ethereum.CallMsg{
			To:   &addr,
			Data: crypto.Keccak256(methodSig)[:4],
		}

		for _, p := range params {
			_ = p
			msg.Data = append(msg.Data, []byte{}...)
		}

		var hex hexutil.Bytes
		if err := h.h.CallContext(
			context.TODO(), &hex, "eth_call", toCallArg(msg), "latest",
		); err != nil {
			// TODO do something with error to give to client
			return
		}

		fmt.Fprintf(w, "contract call result %s\n", hex)
	}
}

type withRPCHandle struct {
	h *rpc.Client
}

func program() error {
	flag.Parse()

	// handle, err := ethclient.Dial(*clientDial)
	handle, err := rpc.Dial(*clientDial)
	if err != nil {
		return err
	}

	router := httprouter.New()
	router.GlobalOPTIONS = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request,
		) {
			if r.Header.Get("Access-Control-Request-Method") != "" {
				// Set CORS headers
				header := w.Header()
				header.Set("Access-Control-Allow-Methods", header.Get("Allow"))
				header.Set("Access-Control-Allow-Origin", "*")
			}
			// Adjust status code to 204
			w.WriteHeader(http.StatusNoContent)
		})

	with := withRPCHandle{
		h: handle,
	}

	router.GET("/", Index)
	router.GET("/:addr/:methodSig/*contractParams", with.evalParams)
	log.Fatal(http.ListenAndServe(":8080", router))
	return nil
}

func main() {
	if err := program(); err != nil {
		fmt.Printf("FATAL: %+v\n", err)
		os.Exit(1)
	}
}
