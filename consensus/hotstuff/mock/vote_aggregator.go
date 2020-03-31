// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"
import model "github.com/dapperlabs/flow-go/consensus/hotstuff/model"

// VoteAggregator is an autogenerated mock type for the VoteAggregator type
type VoteAggregator struct {
	mock.Mock
}

// BuildQCOnReceivedBlock provides a mock function with given fields: block
func (_m *VoteAggregator) BuildQCOnReceivedBlock(block *model.Block) (*model.QuorumCertificate, bool, error) {
	ret := _m.Called(block)

	var r0 *model.QuorumCertificate
	if rf, ok := ret.Get(0).(func(*model.Block) *model.QuorumCertificate); ok {
		r0 = rf(block)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.QuorumCertificate)
		}
	}

	var r1 bool
	if rf, ok := ret.Get(1).(func(*model.Block) bool); ok {
		r1 = rf(block)
	} else {
		r1 = ret.Get(1).(bool)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(*model.Block) error); ok {
		r2 = rf(block)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// PruneByView provides a mock function with given fields: view
func (_m *VoteAggregator) PruneByView(view uint64) {
	_m.Called(view)
}

// StorePendingVote provides a mock function with given fields: vote
func (_m *VoteAggregator) StorePendingVote(vote *model.Vote) bool {
	ret := _m.Called(vote)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*model.Vote) bool); ok {
		r0 = rf(vote)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// StoreProposerVote provides a mock function with given fields: vote
func (_m *VoteAggregator) StoreProposerVote(vote *model.Vote) bool {
	ret := _m.Called(vote)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*model.Vote) bool); ok {
		r0 = rf(vote)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// StoreVoteAndBuildQC provides a mock function with given fields: vote, block
func (_m *VoteAggregator) StoreVoteAndBuildQC(vote *model.Vote, block *model.Block) (*model.QuorumCertificate, bool, error) {
	ret := _m.Called(vote, block)

	var r0 *model.QuorumCertificate
	if rf, ok := ret.Get(0).(func(*model.Vote, *model.Block) *model.QuorumCertificate); ok {
		r0 = rf(vote, block)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.QuorumCertificate)
		}
	}

	var r1 bool
	if rf, ok := ret.Get(1).(func(*model.Vote, *model.Block) bool); ok {
		r1 = rf(vote, block)
	} else {
		r1 = ret.Get(1).(bool)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(*model.Vote, *model.Block) error); ok {
		r2 = rf(vote, block)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}