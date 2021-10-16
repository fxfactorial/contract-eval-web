package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
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

		parsed, err := parse(ps.ByName("methodSig"))
		if err != nil {
			fmt.Fprintf(w, "oops %v", err)
			return
		}

		var (
			args    abi.Arguments
			rawArgs []interface{}
		)

		for _, p := range parsed {
			t, err := abi.NewType(p, "", nil)
			if err != nil {
				fmt.Fprintf(w, "oops %v", err)
				return
			}
			args = append(args, abi.Argument{Type: t})

			switch p {
			case "string":
				rawArgs = append(rawArgs, p)
			case "bytes":
				rawArgs = append(rawArgs, common.Hex2Bytes(p))
			// assume its a number thing
			default:
				num, ok := new(big.Int).SetString(p, 10)
				if !ok {
					fmt.Fprintf(w, "oops this is crap input as number %s", p)
					return
				}
				rawArgs = append(rawArgs, num)
			}
		}

		packed, err := args.Pack(rawArgs...)
		if err != nil {
			fmt.Fprintf(w, "oops %v", err)
			return
		}

		msg.Data = append(msg.Data, packed...)

		var hex hexutil.Bytes
		if err := h.h.CallContext(
			context.TODO(), &hex, "eth_call", toCallArg(msg), "latest",
		); err != nil {
			fmt.Println("some error on eval", err)
			return
		}

		tmpl, _ := template.New("").Parse(`
<!doctype html> 
<head>
 <link rel="stylesheet" href="https/cdnjs.cloudflare.com/ajax/libs/normalize/8.0.1/normalize.min.css">
<title> evalled some code whatever </title>
</head>
<body>
<div> 
  <p> something {{ .HexResult }} </p>
</body>
`)

		if err := tmpl.Execute(w, nil); err != nil {
			fmt.Println("why an issue right", err)
		}

		fmt.Println("contract result", struct{ HexResult string }{HexResult: common.Bytes2Hex(hex)})
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

func parse(s string) ([]string, error) {
	if !sigRegex.Match([]byte(s)) {
		return nil, errors.Errorf("no match %s", s)
	}

	start := strings.Index(s, "(")
	end := strings.Index(s, ")")
	name := s[:start]
	fmt.Println("nam eis ", name)
	params := s[start+1 : end]
	return strings.Split(params, ","), nil
}

// thank you rando gist
var sigRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*\((?:(?:(?:(?:u?)int(?:|8|16|24|32|40|48|56|64|72|80|88|96|104|112|120|128|136|144|152|160|168|176|184|192|200|208|216|224|232|240|248|256)|byte(?:s?)(?:|(?:[1-9]|[12][0-9]|3[0-2]))|address|bool|string)(?:\[(?:\d*)\])*)(?:,(?:(?:(?:u?)int(?:|8|16|24|32|40|48|56|64|72|80|88|96|104|112|120|128|136|144|152|160|168|176|184|192|200|208|216|224|232|240|248|256)|byte(?:s?)(?:|(?:[1-9]|[12][0-9]|3[0-2]))|address|bool|string)+(?:\[(?:\d*)\])*))*)*\)$`)

func main() {

	if err := program(); err != nil {
		fmt.Printf("FATAL: %+v\n", err)
		os.Exit(1)
	}
}
