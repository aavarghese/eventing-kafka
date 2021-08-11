/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package removewithevenpodspreadpriority

import (
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	state "knative.dev/eventing-kafka/pkg/common/scheduler/state"
	tscheduler "knative.dev/eventing-kafka/pkg/common/scheduler/testing"
)

func TestFilter(t *testing.T) {
	testCases := []struct {
		name     string
		state    *state.State
		vpod     types.NamespacedName
		replicas int32
		podID    int32
		expected *state.Status
		expScore uint64
		args     interface{}
	}{
		{
			name:     "no vpods, no pods",
			vpod:     types.NamespacedName{},
			state:    &state.State{StatefulSetName: "pod-name", LastOrdinal: -1, PodSpread: map[types.NamespacedName]map[string]int32{}},
			replicas: 0,
			podID:    0,
			expScore: 0,
			expected: state.NewStatus(state.Success),
			args:     "{\"MaxSkew\": 2}",
		},
		{
			name:     "no vpods, no pods, bad arg",
			vpod:     types.NamespacedName{},
			state:    &state.State{StatefulSetName: "pod-name", LastOrdinal: -1, PodSpread: map[types.NamespacedName]map[string]int32{}},
			replicas: 0,
			podID:    0,
			expScore: 0,
			expected: state.NewStatus(state.Unschedulable, ErrReasonInvalidArg),
			args:     "{\"MaxSkewness\": 2}",
		},
		{
			name: "one vpod, one pod, same pod filter",
			vpod: types.NamespacedName{Name: "vpod-name-0", Namespace: "vpod-ns-0"},
			state: &state.State{StatefulSetName: "pod-name", LastOrdinal: 0,
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: "vpod-name-0", Namespace: "vpod-ns-0"}: {
						"pod-name-0": 5,
					},
				},
			},
			replicas: 1,
			podID:    0,
			expScore: math.MaxUint64,
			expected: state.NewStatus(state.Success),
			args:     "{\"MaxSkew\": 2}",
		},
		{
			name: "two vpods, one pod, same pod filter",
			vpod: types.NamespacedName{Name: "vpod-name-0", Namespace: "vpod-ns-0"},
			state: &state.State{StatefulSetName: "pod-name", LastOrdinal: 0,
				PodSpread: map[types.NamespacedName]map[string]int32{
					{Name: "vpod-name-0", Namespace: "vpod-ns-0"}: {
						"pod-name-0": 5,
					},
					{Name: "vpod-name-1", Namespace: "vpod-ns-1"}: {
						"pod-name-0": 4,
					},
				},
			},
			replicas: 1,
			podID:    0,
			expScore: math.MaxUint64,
			expected: state.NewStatus(state.Success),
			args:     "{\"MaxSkew\": 2}",
		},
		{
			name: "one vpod, two pods,same pod filter",
			vpod: types.NamespacedName{Name: "vpod-name-0", Namespace: "vpod-ns-0"},
			state: &state.State{StatefulSetName: "pod-name", LastOrdinal: 1, PodSpread: map[types.NamespacedName]map[string]int32{
				{Name: "vpod-name-0", Namespace: "vpod-ns-0"}: {
					"pod-name-0": 5,
					"pod-name-1": 5,
				},
			}},
			replicas: 2,
			podID:    1,
			expScore: math.MaxUint64 - 1,
			expected: state.NewStatus(state.Success),
			args:     "{\"MaxSkew\": 2}",
		},
		{
			name: "one vpod, five pods, same pod filter",
			vpod: types.NamespacedName{Name: "vpod-name-0", Namespace: "vpod-ns-0"},
			state: &state.State{StatefulSetName: "pod-name", LastOrdinal: 4, PodSpread: map[types.NamespacedName]map[string]int32{
				{Name: "vpod-name-0", Namespace: "vpod-ns-0"}: {
					"pod-name-0": 5,
					"pod-name-1": 4,
					"pod-name-2": 3,
					"pod-name-3": 4,
					"pod-name-4": 5,
				},
			}},
			replicas: 5,
			podID:    1,
			expScore: math.MaxUint64 - 5,
			expected: state.NewStatus(state.Success),
			args:     "{\"MaxSkew\": 2}",
		},
		{
			name: "one vpod, five pods, same pod filter diff pod",
			vpod: types.NamespacedName{Name: "vpod-name-0", Namespace: "vpod-ns-0"},
			state: &state.State{StatefulSetName: "pod-name", LastOrdinal: 4, PodSpread: map[types.NamespacedName]map[string]int32{
				{Name: "vpod-name-0", Namespace: "vpod-ns-0"}: {
					"pod-name-0": 10,
					"pod-name-1": 4,
					"pod-name-2": 3,
					"pod-name-3": 4,
					"pod-name-4": 5,
				},
			}},
			replicas: 5,
			podID:    0,
			expScore: math.MaxUint64 - 20,
			expected: state.NewStatus(state.Success),
			args:     "{\"MaxSkew\": 2}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := tscheduler.SetupFakeContext(t)
			var plugin = &RemoveWithEvenPodSpreadPriority{}

			name := plugin.Name()
			assert.Equal(t, name, state.RemoveWithEvenPodSpreadPriority)

			score, status := plugin.Score(ctx, tc.args, tc.state, tc.vpod, tc.podID)
			if score != tc.expScore {
				t.Errorf("unexpected score, got %v, want %v", score, tc.expScore)
			}
			if !reflect.DeepEqual(status, tc.expected) {
				t.Errorf("unexpected status, got %v, want %v", status, tc.expected)
			}
		})
	}
}
