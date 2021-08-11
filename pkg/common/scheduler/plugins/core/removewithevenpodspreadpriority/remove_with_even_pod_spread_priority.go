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
	"context"
	"encoding/json"
	"math"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing-kafka/pkg/common/scheduler/factory"
	state "knative.dev/eventing-kafka/pkg/common/scheduler/state"
	"knative.dev/pkg/logging"
)

// RemoveWithEvenPodSpreadPriority is a filter plugin that eliminates pods that do not create an equal spread of resources across pods
type RemoveWithEvenPodSpreadPriority struct {
}

// Verify RemoveWithEvenPodSpreadPriority Implements FilterPlugin Interface
var _ state.ScorePlugin = &RemoveWithEvenPodSpreadPriority{}

// Name of the plugin
const (
	Name                   = state.RemoveWithEvenPodSpreadPriority
	ErrReasonInvalidArg    = "invalid arguments"
	ErrReasonUnschedulable = "pod will cause an uneven spread"
)

func init() {
	factory.RegisterSP(Name, &RemoveWithEvenPodSpreadPriority{})
}

// Name returns name of the plugin
func (pl *RemoveWithEvenPodSpreadPriority) Name() string {
	return Name
}

// Score invoked at the score extension point.
func (pl *RemoveWithEvenPodSpreadPriority) Score(ctx context.Context, args interface{}, states *state.State, key types.NamespacedName, podID int32) (uint64, *state.Status) {
	logger := logging.FromContext(ctx).With("Score", pl.Name())
	var score uint64 = 0

	spreadArgs, ok := args.(string)
	if !ok {
		logger.Errorf("Scoring args %v for priority %q are not valid", args, pl.Name())
		return 0, state.NewStatus(state.Unschedulable, ErrReasonInvalidArg)
	}

	skewVal := state.EvenPodSpreadArgs{}
	decoder := json.NewDecoder(strings.NewReader(spreadArgs))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&skewVal); err != nil {
		return 0, state.NewStatus(state.Unschedulable, ErrReasonInvalidArg)
	}

	if states.LastOrdinal >= 0 { //need atleast two pods to compute spread
		currentReps := states.PodSpread[key][state.PodNameFromOrdinal(states.StatefulSetName, podID)] //get #vreps on this podID
		var skew int32
		for otherPodID := int32(0); otherPodID <= states.LastOrdinal; otherPodID++ { //compare with #vreps on other pods
			if otherPodID != podID {
				otherReps := states.PodSpread[key][state.PodNameFromOrdinal(states.StatefulSetName, otherPodID)]
				if skew = (currentReps - 1) - otherReps; skew < 0 {
					skew = skew * int32(-1)
				}

				logger.Infof("Current Pod %v with %d and Other Pod %v with %d causing skew %d", podID, currentReps, otherPodID, otherReps, skew)
				if skew > skewVal.MaxSkew { //score low
					logger.Infof("Pod %d will cause an uneven zone spread", podID)
				}
				score = score + uint64(skew)
			}
		}
		score = math.MaxUint64 - score //lesser skews get higher score
	}

	logger.Infof("Pod %v scored by %q priority successfully with score %v", podID, pl.Name(), score)

	return score, state.NewStatus(state.Success)
}

// ScoreExtensions of the Score plugin.
func (pl *RemoveWithEvenPodSpreadPriority) ScoreExtensions() state.ScoreExtensions {
	return pl
}

// NormalizeScore invoked after scoring all pods. Calculates the score of each pod
// based on the number of existing vreplicas on the pods. It favors pods
// in zones that create an even spread of resources for HA
func (pl *RemoveWithEvenPodSpreadPriority) NormalizeScore(ctx context.Context, states *state.State, scores state.PodScoreList) *state.Status {
	return nil
}
