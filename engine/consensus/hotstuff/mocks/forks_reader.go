// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	flow "github.com/dapperlabs/flow-go/model/flow"
	hotstuff "github.com/dapperlabs/flow-go/model/hotstuff"

	mock "github.com/stretchr/testify/mock"
)

// ForksReader is an autogenerated mock type for the ForksReader type
type ForksReader struct {
	mock.Mock
}

// FinalizedBlock provides a mock function with given fields:
func (_m *ForksReader) FinalizedBlock() *hotstuff.Block {
	ret := _m.Called()

	var r0 *hotstuff.Block
	if rf, ok := ret.Get(0).(func() *hotstuff.Block); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*hotstuff.Block)
		}
	}

	return r0
}

// FinalizedView provides a mock function with given fields:
func (_m *ForksReader) FinalizedView() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// GetBlock provides a mock function with given fields: id
func (_m *ForksReader) GetBlock(id flow.Identifier) (*hotstuff.Block, bool) {
	ret := _m.Called(id)

	var r0 *hotstuff.Block
	if rf, ok := ret.Get(0).(func(flow.Identifier) *hotstuff.Block); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*hotstuff.Block)
		}
	}

	var r1 bool
	if rf, ok := ret.Get(1).(func(flow.Identifier) bool); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// GetBlocksForView provides a mock function with given fields: view
func (_m *ForksReader) GetBlocksForView(view uint64) []*hotstuff.Block {
	ret := _m.Called(view)

	var r0 []*hotstuff.Block
	if rf, ok := ret.Get(0).(func(uint64) []*hotstuff.Block); ok {
		r0 = rf(view)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*hotstuff.Block)
		}
	}

	return r0
}

// IsSafeBlock provides a mock function with given fields: block
func (_m *ForksReader) IsSafeBlock(block *hotstuff.Block) bool {
	ret := _m.Called(block)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*hotstuff.Block) bool); ok {
		r0 = rf(block)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}