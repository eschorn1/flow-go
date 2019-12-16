// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import (
	flow "github.com/dapperlabs/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// Conduit is an autogenerated mock type for the Conduit type
type Conduit struct {
	mock.Mock
}

// Submit provides a mock function with given fields: event, targetIDs
func (_m *Conduit) Submit(event interface{}, targetIDs ...flow.Identifier) error {
	_va := make([]interface{}, len(targetIDs))
	for _i := range targetIDs {
		_va[_i] = targetIDs[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, event)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}, ...flow.Identifier) error); ok {
		r0 = rf(event, targetIDs...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
