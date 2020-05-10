// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import flow "github.com/dapperlabs/flow-go/model/flow"
import mock "github.com/stretchr/testify/mock"

// VerificationMetrics is an autogenerated mock type for the VerificationMetrics type
type VerificationMetrics struct {
	mock.Mock
}

// OnChunkDataAdded provides a mock function with given fields: chunkID, size
func (_m *VerificationMetrics) OnChunkDataAdded(chunkID flow.Identifier, size float64) {
	_m.Called(chunkID, size)
}

// OnChunkDataRemoved provides a mock function with given fields: chunkID, size
func (_m *VerificationMetrics) OnChunkDataRemoved(chunkID flow.Identifier, size float64) {
	_m.Called(chunkID, size)
}

// OnChunkVerificationFinished provides a mock function with given fields: chunkID
func (_m *VerificationMetrics) OnChunkVerificationFinished(chunkID flow.Identifier) {
	_m.Called(chunkID)
}

// OnChunkVerificationStarted provides a mock function with given fields: chunkID
func (_m *VerificationMetrics) OnChunkVerificationStarted(chunkID flow.Identifier) {
	_m.Called(chunkID)
}

// OnResultApproval provides a mock function with given fields:
func (_m *VerificationMetrics) OnResultApproval() {
	_m.Called()
}