package main

import (
	"fmt"

	ponsparser "github.com/0xfnzero/pons-parser-sdk/ponsparser"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func main() {
	receipt := &types.Receipt{}
	factory := common.HexToAddress("0x7eD598BcEf8bd9Edd8C97A195C6d13f40801EC7e")
	validator := func(kind ponsparser.EventKind, emitter common.Address) bool {
		return kind == ponsparser.EventTokenLaunched && emitter == factory
	}
	events, err := ponsparser.ParseReceiptEventsFiltered(receipt, validator)
	if err != nil {
		panic(err)
	}
	fmt.Println("parsed events:", len(events))
}
