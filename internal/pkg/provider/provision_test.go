// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPickNode(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		input    []nodeStatus
	}{
		{
			name: "Single node should be returned",
			input: []nodeStatus{
				{Name: "node1", MemoryFree: 1, SameMachineRequestSetVMs: 0},
			},
			expected: "node1",
		},
		{
			name: "Primary criteria: Pick node with fewer same-set VMs",
			input: []nodeStatus{
				{Name: "NodeA", MemoryFree: 1.0, SameMachineRequestSetVMs: 10},
				{Name: "NodeB", MemoryFree: 0.5, SameMachineRequestSetVMs: 0},
			},
			expected: "NodeB",
		},
		{
			name: "Pending reservations outweigh memory when effective counts differ",
			input: []nodeStatus{
				{Name: "NodeA", MemoryFree: 1.0, SameMachineRequestSetVMs: 1},
				{Name: "NodeB", MemoryFree: 0.5, SameMachineRequestSetVMs: 0},
			},
			expected: "NodeB",
		},
		{
			name: "Secondary criteria: If VMs equal, pick node with MOST free memory",
			input: []nodeStatus{
				{Name: "NodeA", MemoryFree: 0.5, SameMachineRequestSetVMs: 5},
				{Name: "NodeB", MemoryFree: 1.0, SameMachineRequestSetVMs: 5},
				{Name: "NodeC", MemoryFree: 0.1, SameMachineRequestSetVMs: 5},
			},
			expected: "NodeB",
		},
		{
			name: "Complex scenario",
			input: []nodeStatus{
				{Name: "NodeA", MemoryFree: 0.1, SameMachineRequestSetVMs: 2},
				{Name: "NodeB", MemoryFree: 0.05, SameMachineRequestSetVMs: 1},
				{Name: "NodeC", MemoryFree: 0.04, SameMachineRequestSetVMs: 1},
				{Name: "NodeD", MemoryFree: 1, SameMachineRequestSetVMs: 5},
			},
			expected: "NodeB",
		},
		{
			name: "No free memory",
			input: []nodeStatus{
				{Name: "NodeA", MemoryFree: 0, SameMachineRequestSetVMs: 0},
				{Name: "NodeB", MemoryFree: 1, SameMachineRequestSetVMs: 0},
			},
			expected: "NodeB",
		},
		{
			name: "Final tie break is deterministic by node name",
			input: []nodeStatus{
				{Name: "NodeB", MemoryFree: 0.5, SameMachineRequestSetVMs: 1},
				{Name: "NodeA", MemoryFree: 0.5, SameMachineRequestSetVMs: 1},
			},
			expected: "NodeA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pickNode(tt.input)

			require.Equal(t, tt.expected, result.Name)
		})
	}
}

func TestNewNodeStatus(t *testing.T) {
	t.Run("online nodes with valid memory are candidates", func(t *testing.T) {
		status, ok := newNodeStatus("node-a", "online", 100, 25)

		require.True(t, ok)
		require.Equal(t, "node-a", status.Name)
		require.InDelta(t, 0.75, status.MemoryFree, 0.0001)
	})

	t.Run("offline nodes are excluded", func(t *testing.T) {
		_, ok := newNodeStatus("node-a", "offline", 100, 25)

		require.False(t, ok)
	})

	t.Run("nodes without valid memory totals are excluded", func(t *testing.T) {
		_, ok := newNodeStatus("node-a", "online", 0, 0)

		require.False(t, ok)
	})

	t.Run("memory usage above max is clamped", func(t *testing.T) {
		status, ok := newNodeStatus("node-a", "online", 100, 150)

		require.True(t, ok)
		require.Zero(t, status.MemoryFree)
	})
}

func TestNodeReservations(t *testing.T) {
	p := NewProvisioner(nil)

	p.reserveNodeReservation("set-a", "req-1", "node-a")
	p.reserveNodeReservation("set-a", "req-1", "node-a")
	p.reserveNodeReservation("set-a", "req-2", "node-a")
	p.reserveNodeReservation("set-a", "req-3", "node-b")
	p.reserveNodeReservation("set-b", "req-4", "node-a")

	reservation, ok := p.getNodeReservation("req-1")
	require.True(t, ok)
	require.Equal(t, "set-a", reservation.MachineRequestSetID)
	require.Equal(t, "node-a", reservation.Node)

	require.Equal(t, 1, p.countNodeReservations("set-a", "req-1", "node-a"))
	require.Equal(t, 0, p.countNodeReservations("set-a", "req-3", "node-b"))
	require.Equal(t, 0, p.countNodeReservations("set-b", "req-4", "node-a"))

	p.releaseNodeReservation("req-2")

	require.Equal(t, 0, p.countNodeReservations("set-a", "req-1", "node-a"))
}
