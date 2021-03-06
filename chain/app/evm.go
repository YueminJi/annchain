// Copyright 2017 Annchain Information Technology Services Co.,Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"fmt"
	"math/big"

	"github.com/annchain/annchain/tools"
	"github.com/annchain/annchain/types"
	ethcmn "github.com/annchain/anth/common"
	ethcore "github.com/annchain/anth/core"
	ethtypes "github.com/annchain/anth/core/types"
	"github.com/annchain/anth/rlp"
)

var (
	ReceiptsPrefix = []byte("receipts-")
)

// ExecuteEVMTx execute tx one by one in the loop, without lock, so should always be called between Lock() and Unlock() on the *stateDup
func (app *App) ExecuteEVMTx(header *ethtypes.Header, blockHash ethcmn.Hash, tx *types.BlockTx, txIndex int) (hash []byte, usedGas *big.Int, err error) {
	stateSnapshot := app.currentEvmState.Snapshot()

	txBody := &types.TxEvmCommon{}
	if err = tools.FromBytes(tx.Payload, txBody); err != nil {
		return
	}

	txHash := tools.Hash(tx)
	from := ethcmn.BytesToAddress(tx.Sender)
	var to ethcmn.Address
	var evmTx *ethtypes.Transaction
	if len(txBody.To) == 0 {
		evmTx = ethtypes.NewContractCreation(tx.Nonce, from, txBody.Amount, tx.GasLimit, tx.GasPrice, txBody.Load)
	} else {
		to = ethcmn.BytesToAddress(txBody.To)
		evmTx = ethtypes.NewTransaction(tx.Nonce, from, to, txBody.Amount, tx.GasLimit, tx.GasPrice, txBody.Load)
	}
	gp := new(ethcore.GasPool).AddGas(header.GasLimit)
	app.currentEvmState.StartRecord(ethcmn.BytesToHash(txHash), blockHash, txIndex)
	receipt, usedGas, err := ethcore.ApplyTransaction(
		app.chainConfig,
		nil,
		gp,
		app.currentEvmState,
		header,
		evmTx,
		txHash,
		big.NewInt(0),
		evmConfig)

	if err != nil {
		app.currentEvmState.RevertToSnapshot(stateSnapshot)
		return
	}

	if receipt != nil {
		app.receipts = append(app.receipts, receipt)
	}

	return txHash, usedGas, err
}

func (app *App) CheckEVMTx(bs []byte) error {
	tx := new(types.BlockTx)
	err := rlp.DecodeBytes(bs, tx)
	if err != nil {
		return err
	}
	err = tools.VerifySecp256k1(tx, tx.Sender, tx.Signature)
	if err != nil {
		return err
	}

	from := ethcmn.BytesToAddress(tx.Sender)
	app.evmStateMtx.Lock()
	defer app.evmStateMtx.Unlock()
	if app.evmState.GetNonce(from) > tx.Nonce {
		return fmt.Errorf("nonce too low")
	}
	// if app.evmState.GetBalance(from).Cmp(tx.Cost()) < 0 {
	// 	return fmt.Errorf("not enough funds")
	// }
	return nil
}
