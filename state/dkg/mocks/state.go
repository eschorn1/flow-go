// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import crypto "github.com/dapperlabs/flow-go/crypto"

import flow "github.com/dapperlabs/flow-go/model/flow"
import mock "github.com/stretchr/testify/mock"

// State is an autogenerated mock type for the State type
type State struct {
	mock.Mock
}

// GroupKey provides a mock function with given fields:
func (_m *State) GroupKey() (crypto.PublicKey, error) {
	ret := _m.Called()

	var r0 crypto.PublicKey
	if rf, ok := ret.Get(0).(func() crypto.PublicKey); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(crypto.PublicKey)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GroupSize provides a mock function with given fields:
func (_m *State) GroupSize() (uint, error) {
	ret := _m.Called()

	var r0 uint
	if rf, ok := ret.Get(0).(func() uint); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// HasParticipant provides a mock function with given fields: nodeID
func (_m *State) HasParticipant(nodeID flow.Identifier) (bool, error) {
	ret := _m.Called(nodeID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(nodeID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(nodeID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ParticipantIndex provides a mock function with given fields: nodeID
func (_m *State) ParticipantIndex(nodeID flow.Identifier) (uint, error) {
	ret := _m.Called(nodeID)

	var r0 uint
	if rf, ok := ret.Get(0).(func(flow.Identifier) uint); ok {
		r0 = rf(nodeID)
	} else {
		r0 = ret.Get(0).(uint)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(nodeID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ParticipantKey provides a mock function with given fields: nodeID
func (_m *State) ParticipantKey(nodeID flow.Identifier) (crypto.PublicKey, error) {
	ret := _m.Called(nodeID)

	var r0 crypto.PublicKey
	if rf, ok := ret.Get(0).(func(flow.Identifier) crypto.PublicKey); ok {
		r0 = rf(nodeID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(crypto.PublicKey)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(nodeID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}