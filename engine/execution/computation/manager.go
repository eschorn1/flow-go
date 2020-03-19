package computation

import (
	"fmt"

	"github.com/dapperlabs/cadence"
	"github.com/dapperlabs/cadence/encoding"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/computation/computer"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

type ComputationManager interface {
	ExecuteScript([]byte, *flow.Header, *state.View) ([]byte, error)
	ComputeBlock(block *entity.ExecutableBlock, view *state.View) (*execution.ComputationResult, error)
}

// Manager manages computation and execution
type Manager struct {
	log           zerolog.Logger
	me            module.Local
	protoState    protocol.State
	vm            virtualmachine.VirtualMachine
	blockComputer computer.BlockComputer
}

func New(
	logger zerolog.Logger,
	me module.Local,
	protoState protocol.State,
	vm virtualmachine.VirtualMachine,
) *Manager {
	log := logger.With().Str("engine", "computation").Logger()

	e := Manager{
		log:           log,
		me:            me,
		protoState:    protoState,
		vm:            vm,
		blockComputer: computer.NewBlockComputer(vm),
	}

	return &e
}

func (e *Manager) ExecuteScript(script []byte, blockHeader *flow.Header, view *state.View) ([]byte, error) {

	result, err := e.vm.NewBlockContext(blockHeader).ExecuteScript(view, script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute script (internal error): %w", err)
	}

	if !result.Succeeded() {
		return nil, fmt.Errorf("failed to execute script: %w", result.Error)
	}

	value, err := cadence.ConvertValue(result.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to export runtime value: %w", err)
	}

	encodedValue, err := encoding.Encode(value)
	if err != nil {
		return nil, fmt.Errorf("failed to encode runtime value: %w", err)
	}

	return encodedValue, nil
}

func (e *Manager) ComputeBlock(block *entity.ExecutableBlock, view *state.View) (*execution.ComputationResult, error) {
	e.log.Debug().
		Hex("block_id", logging.Entity(block.Block)).
		Msg("received complete block")

	result, err := e.blockComputer.ExecuteBlock(block, view)
	if err != nil {
		e.log.Error().
			Hex("block_id", logging.Entity(block.Block)).
			Msg("failed to compute block result")

		return nil, fmt.Errorf("failed to execute block: %w", err)
	}

	e.log.Debug().
		Hex("block_id", logging.Entity(result.ExecutableBlock.Block)).
		Msg("computed block result")

	return result, nil
}