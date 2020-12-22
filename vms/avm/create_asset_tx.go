// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package avm

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

const (
	minNameLen      = 1
	maxNameLen      = 128
	minSymbolLen    = 1
	maxSymbolLen    = 4
	maxDenomination = 32
)

var (
	errInitialStatesNotSortedUnique = errors.New("initial states not sorted and unique")
	errNameTooShort                 = fmt.Errorf("name is too short, minimum size is %d", minNameLen)
	errNameTooLong                  = fmt.Errorf("name is too long, maximum size is %d", maxNameLen)
	errSymbolTooShort               = fmt.Errorf("symbol is too short, minimum size is %d", minSymbolLen)
	errSymbolTooLong                = fmt.Errorf("symbol is too long, maximum size is %d", maxSymbolLen)
	errNoFxs                        = errors.New("assets must support at least one Fx")
	errIllegalNameCharacter         = errors.New("asset's name must be made up of only letters and numbers")
	errIllegalSymbolCharacter       = errors.New("asset's symbol must be all upper case letters")
	errUnexpectedWhitespace         = errors.New("unexpected whitespace provided")
	errDenominationTooLarge         = errors.New("denomination is too large")
	errMultipleManagers             = errors.New("asset can't have multiple managers")
	errCreateManagedAssetBadCodec   = errors.New("create managed asset tx has invalid codec version")
)

// CreateAssetTx is a transaction that creates a new asset.
type CreateAssetTx struct {
	BaseTx       `serialize:"true"`
	Name         string          `serialize:"true" json:"name"`
	Symbol       string          `serialize:"true" json:"symbol"`
	Denomination byte            `serialize:"true" json:"denomination"`
	States       []*InitialState `serialize:"true" json:"initialStates"`
}

// InitialStates track which virtual machines, and the initial state of these
// machines, this asset uses. The returned array should not be modified.
func (t *CreateAssetTx) InitialStates() []*InitialState { return t.States }

// UTXOs returns the UTXOs transaction is producing.
func (t *CreateAssetTx) UTXOs() []*avax.UTXO {
	txID := t.ID()
	utxos := t.BaseTx.UTXOs()

	for _, state := range t.States {
		for _, out := range state.Outs {
			utxos = append(utxos, &avax.UTXO{
				UTXOID: avax.UTXOID{
					TxID:        txID,
					OutputIndex: uint32(len(utxos)),
				},
				Asset: avax.Asset{
					ID: txID,
				},
				Out: out,
			})
		}
	}
	return utxos
}

// SyntacticVerify that this transaction is well-formed.
func (t *CreateAssetTx) SyntacticVerify(
	ctx *snow.Context,
	c codec.Manager,
	codecVersion uint16,
	txFeeAssetID ids.ID,
	_ uint64,
	txFee uint64,
	numFxs int,
) error {
	switch {
	case t == nil:
		return errNilTx
	case len(t.Name) < minNameLen:
		return errNameTooShort
	case len(t.Name) > maxNameLen:
		return errNameTooLong
	case len(t.Symbol) < minSymbolLen:
		return errSymbolTooShort
	case len(t.Symbol) > maxSymbolLen:
		return errSymbolTooLong
	case len(t.States) == 0:
		return errNoFxs
	case t.Denomination > maxDenomination:
		return errDenominationTooLarge
	case strings.TrimSpace(t.Name) != t.Name:
		return errUnexpectedWhitespace
	}

	for _, r := range t.Name {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ') {
			return errIllegalNameCharacter
		}
	}
	for _, r := range t.Symbol {
		if r > unicode.MaxASCII || !unicode.IsUpper(r) {
			return errIllegalSymbolCharacter
		}
	}

	if err := t.BaseTx.SyntacticVerify(
		ctx,
		c,
		codecVersion,
		txFeeAssetID,
		txFee,
		txFee,
		numFxs,
	); err != nil {
		return err
	}

	for _, state := range t.States {
		if err := state.Verify(c, codecVersion, numFxs); err != nil {
			return err
		}
	}
	if !isSortedAndUniqueInitialStates(t.States) {
		return errInitialStatesNotSortedUnique
	}

	// An asset can have at most one ManagedAssetStatusOutput
	hasManager := false
	for _, state := range t.States {
		if state.FxID != 0 { // TODO lookup secp fx ID
			continue
		}
		for _, out := range state.Outs {
			if _, ok := out.(*secp256k1fx.ManagedAssetStatusOutput); ok {
				if hasManager {
					return errMultipleManagers
				}
				hasManager = true
			}
		}
	}
	// Make sure the codec version is at or after the Apricot fork codec
	if hasManager && codecVersion == preApricotCodecVersion {
		return errCreateManagedAssetBadCodec
	}

	return nil
}

// Sort ...
func (t *CreateAssetTx) Sort() { sortInitialStates(t.States) }
