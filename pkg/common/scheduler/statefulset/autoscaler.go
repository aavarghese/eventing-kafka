/*
Copyright 2020 The Knative Authors

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

package statefulset

import (
	"context"
	"math"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	"knative.dev/eventing-kafka/pkg/common/scheduler"
	st "knative.dev/eventing-kafka/pkg/common/scheduler/state"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

type Autoscaler interface {
	// Start runs the autoscaler until cancelled.
	Start(ctx context.Context)

	// Autoscale is used to immediately trigger the autoscaler with the hint
	// that pending number of vreplicas couldn't be scheduled.
	Autoscale(pending int32)
}

type autoscaler struct {
	statefulSetClient clientappsv1.StatefulSetInterface
	statefulSetName   string
	vpodLister        scheduler.VPodLister
	logger            *zap.SugaredLogger
	stateAccessor     st.StateAccessor
	trigger           chan int32
	evictor           scheduler.Evictor

	// capacity is the total number of virtual replicas available per pod.
	capacity int32

	// refreshPeriod is how often the autoscaler tries to scale down the statefulset
	refreshPeriod time.Duration
}

func NewAutoscaler(ctx context.Context,
	namespace, name string,
	lister scheduler.VPodLister,
	stateAccessor st.StateAccessor,
	evictor scheduler.Evictor,
	refreshPeriod time.Duration,
	capacity int32) Autoscaler {

	return &autoscaler{
		logger:            logging.FromContext(ctx),
		statefulSetClient: kubeclient.Get(ctx).AppsV1().StatefulSets(namespace),
		statefulSetName:   name,
		vpodLister:        lister,
		stateAccessor:     stateAccessor,
		evictor:           evictor,
		trigger:           make(chan int32, 1),
		capacity:          capacity,
		refreshPeriod:     refreshPeriod,
	}
}

func (a *autoscaler) Start(ctx context.Context) {
	attemptScaleDown := false
	pending := int32(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(a.refreshPeriod):
			attemptScaleDown = true
		case pending = <-a.trigger:
			attemptScaleDown = false
		}

		// Retry a few times, just so that we don't have to wait for the next beat when
		// a transient error occurs
		wait.Poll(500*time.Millisecond, 5*time.Second, func() (bool, error) {
			err := a.doautoscale(ctx, attemptScaleDown, pending)
			return err == nil, nil
		})

		pending = int32(0)
	}
}

func (a *autoscaler) Autoscale(pending int32) {
	a.trigger <- pending
}

func (a *autoscaler) doautoscale(ctx context.Context, attemptScaleDown bool, pending int32) error {
	state, err := a.stateAccessor.State(nil)
	if err != nil {
		a.logger.Info("error while refreshing scheduler state (will retry)", zap.Error(err))
		return err
	}

	scale, err := a.statefulSetClient.GetScale(ctx, a.statefulSetName, metav1.GetOptions{})
	if err != nil {
		// skip a beat
		a.logger.Infow("failed to get scale subresource", zap.Error(err))
		return err
	}

	a.logger.Infow("checking adapter capacity",
		zap.Int32("pending", pending),
		zap.Int32("replicas", scale.Spec.Replicas),
		zap.Int32("last ordinal", state.LastOrdinal))

	var scaleUpFactor, newreplicas int32
	if state.SchedPolicy != nil && contains(state.SchedPolicy.Priorities, st.AvailabilityZonePriority) { //HA scaling across zones
		scaleUpFactor = state.NumZones
	} else if state.SchedPolicy != nil && contains(state.SchedPolicy.Priorities, st.AvailabilityNodePriority) { //HA scalingacross nodes
		scaleUpFactor = state.NumNodes
	} else {
		scaleUpFactor = 1 // Non-HA scaling
	}

	newreplicas = state.LastOrdinal + 1 // Ideal number

	// Take into account pending replicas
	if pending > 0 {
		// Make sure to allocate enough pods for holding all pending replicas.
		minNumPods := math.Ceil(float64(pending) / float64(a.capacity))
		newreplicas += int32(math.Ceil(minNumPods/float64(scaleUpFactor)) * float64(scaleUpFactor))
	}

	// Make sure to never scale down past the last ordinal
	if newreplicas <= state.LastOrdinal {
		newreplicas = state.LastOrdinal + scaleUpFactor
	}

	// Only scale down if permitted
	if !attemptScaleDown && newreplicas < scale.Spec.Replicas {
		newreplicas = scale.Spec.Replicas
	}

	if newreplicas != scale.Spec.Replicas {
		scale.Spec.Replicas = newreplicas
		a.logger.Infow("updating adapter replicas", zap.Int32("replicas", scale.Spec.Replicas))

		_, err = a.statefulSetClient.UpdateScale(ctx, a.statefulSetName, scale, metav1.UpdateOptions{})
		if err != nil {
			a.logger.Errorw("updating scale subresource failed", zap.Error(err))
			return err
		}
	} else if attemptScaleDown {
		// since the number of replicas hasn't changed and time has approached to scale down,
		// take the opportunity to compact the vreplicas
		a.mayCompact(state, scaleUpFactor)
	}

	return nil
}

func (a *autoscaler) mayCompact(s *st.State, scaleUpFactor int32) {
	// when there is only one pod there is nothing to move!
	if s.LastOrdinal < 1 {
		return
	}

	if s.SchedulerPolicy == scheduler.MAXFILLUP {
		// Determine if there is enough free capacity to
		// move all vreplicas placed in the last pod to pods with a lower ordinal
		freeCapacity := s.FreeCapacity() - s.Free(s.LastOrdinal)
		usedInLastPod := s.Capacity - s.Free(s.LastOrdinal)

		if freeCapacity >= usedInLastPod {
			err := a.compact(s, scaleUpFactor)
			if err != nil {
				a.logger.Errorw("vreplicas compaction failed", zap.Error(err))
			}
		}

		// only do 1 replica at a time to avoid overloading the scheduler with too many
		// rescheduling requests.
	} else if s.SchedPolicy != nil {
		freeCapacity := s.FreeCapacity()
		usedInLastXPods := s.Capacity * scaleUpFactor
		for i := int32(0); i < scaleUpFactor && s.LastOrdinal-i >= 0; i++ {
			freeCapacity = freeCapacity - s.Free(s.LastOrdinal-i)
			usedInLastXPods = usedInLastXPods - s.Free(s.LastOrdinal-i)
		}

		if freeCapacity >= usedInLastXPods {
			err := a.compact(s, scaleUpFactor)
			if err != nil {
				a.logger.Errorw("vreplicas compaction failed", zap.Error(err))
			}
		}
	}
}

func (a *autoscaler) compact(s *st.State, scaleUpFactor int32) error {
	vpods, err := a.vpodLister()
	if err != nil {
		return err
	}

	for _, vpod := range vpods {
		placements := vpod.GetPlacements()
		for _, placement := range placements {
			for i := int32(0); i < scaleUpFactor; i++ {
				ordinal := st.OrdinalFromPodName(placement.PodName)

				if ordinal == s.LastOrdinal-i {
					err = a.evictor(vpod, &placement)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func contains(priors []scheduler.PriorityPolicy, name string) bool {
	for _, v := range priors {
		if v.Name == name {
			return true
		}
	}

	return false
}
