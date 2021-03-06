package contracts

import (
	"math/big"

	"github.com/smartcontractkit/chainlink/core/services/eth"
	"github.com/smartcontractkit/chainlink/core/services/log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
)

//go:generate mockery --name FluxAggregator --output ../../../internal/mocks/ --case=underscore

type FluxAggregator interface {
	ConnectedContract
	RoundState(oracle common.Address, roundID uint32) (FluxAggregatorRoundState, error)
	GetOracles() ([]common.Address, error)
	LatestRoundData() (FluxAggregatorRoundData, error)
}

const (
	// FluxAggregatorName is the name of Chainlink's Ethereum contract for
	// aggregating numerical data such as prices.
	FluxAggregatorName = "FluxAggregator"
)

var (
	// AggregatorNewRoundLogTopic20191220 is the NewRound filter topic for
	// the FluxAggregator as of Dec. 20th 2019. Eagerly fails if not found.
	AggregatorNewRoundLogTopic20191220 = eth.MustGetV6ContractEventID("FluxAggregator", "NewRound")
	// AggregatorAnswerUpdatedLogTopic20191220 is the AnswerUpdated filter topic for
	// the FluxAggregator as of Dec. 20th 2019. Eagerly fails if not found.
	AggregatorAnswerUpdatedLogTopic20191220 = eth.MustGetV6ContractEventID("FluxAggregator", "AnswerUpdated")
)

type fluxAggregator struct {
	ConnectedContract
	ethClient eth.Client
	address   common.Address
}

type LogNewRound struct {
	types.Log
	RoundId   *big.Int
	StartedBy common.Address
	// seconds since unix epoch
	StartedAt *big.Int
}

type LogAnswerUpdated struct {
	types.Log
	Current   *big.Int
	RoundId   *big.Int
	UpdatedAt *big.Int
}

var fluxAggregatorLogTypes = map[common.Hash]interface{}{
	AggregatorNewRoundLogTopic20191220:      &LogNewRound{},
	AggregatorAnswerUpdatedLogTopic20191220: &LogAnswerUpdated{},
}

func NewFluxAggregator(address common.Address, ethClient eth.Client, logBroadcaster log.Broadcaster) (FluxAggregator, error) {
	codec, err := eth.GetV6ContractCodec(FluxAggregatorName)
	if err != nil {
		return nil, err
	}
	connectedContract := NewConnectedContract(codec, address, ethClient, logBroadcaster)
	return &fluxAggregator{connectedContract, ethClient, address}, nil
}

func (fa *fluxAggregator) SubscribeToLogs(listener log.Listener) (connected bool, _ UnsubscribeFunc) {
	return fa.ConnectedContract.SubscribeToLogs(
		log.NewDecodingLogListener(fa, fluxAggregatorLogTypes, listener),
	)
}

type FluxAggregatorRoundState struct {
	ReportableRoundID uint32   `abi:"_roundId" json:"reportableRoundID"`
	EligibleToSubmit  bool     `abi:"_eligibleToSubmit" json:"eligibleToSubmit"`
	LatestAnswer      *big.Int `abi:"_latestSubmission" json:"latestAnswer,omitempty"`
	Timeout           uint64   `abi:"_timeout" json:"timeout"`
	StartedAt         uint64   `abi:"_startedAt" json:"startedAt"`
	AvailableFunds    *big.Int `abi:"_availableFunds" json:"availableFunds,omitempty"`
	PaymentAmount     *big.Int `abi:"_paymentAmount" json:"paymentAmount,omitempty"`
	OracleCount       uint8    `abi:"_oracleCount" json:"oracleCount"`
}

type FluxAggregatorRoundData struct {
	RoundID         *big.Int `abi:"roundId" json:"reportableRoundID"`
	Answer          *big.Int `abi:"answer" json:"latestAnswer,omitempty"`
	StartedAt       *big.Int `abi:"startedAt" json:"startedAt"`
	UpdatedAt       *big.Int `abi:"updatedAt" json:"updatedAt"`
	AnsweredInRound *big.Int `abi:"answeredInRound" json:"availableFunds,omitempty"`
}

func (rs FluxAggregatorRoundState) TimesOutAt() uint64 {
	return rs.StartedAt + rs.Timeout
}

func (fa *fluxAggregator) RoundState(oracle common.Address, roundID uint32) (FluxAggregatorRoundState, error) {
	var result FluxAggregatorRoundState
	err := fa.Call(&result, "oracleRoundState", oracle, roundID)
	if err != nil {
		return FluxAggregatorRoundState{}, errors.Wrap(err, "unable to encode message call")
	}
	return result, nil
}

func (fa *fluxAggregator) GetOracles() (oracles []common.Address, err error) {
	oracles = make([]common.Address, 0)
	err = fa.Call(&oracles, "getOracles")
	if err != nil {
		return nil, errors.Wrap(err, "error calling flux aggregator getOracles")
	}
	return oracles, nil
}

func (fa *fluxAggregator) LatestRoundData() (FluxAggregatorRoundData, error) {
	var result FluxAggregatorRoundData
	err := fa.Call(&result, "latestRoundData")
	if err != nil {
		return FluxAggregatorRoundData{},
			errors.Wrap(err, "error calling fluxaggregator#latestRoundData - contract may have 0 rounds")
	}
	return result, nil
}
