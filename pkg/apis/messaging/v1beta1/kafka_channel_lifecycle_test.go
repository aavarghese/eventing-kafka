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

package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func init() {
	// Initialize A Default ConditionSet For Testing
	RegisterAlternateKafkaChannelConditionSet(apis.NewLivingConditionSet(
		KafkaChannelConditionAddressable,
		KafkaChannelConditionConfigReady,
		KafkaChannelConditionTopicReady,
		KafkaChannelConditionChannelServiceReady))
}

var condReady = apis.Condition{
	Type:   KafkaChannelConditionReady,
	Status: corev1.ConditionTrue,
}

var condTopicNotReady = apis.Condition{
	Type:   KafkaChannelConditionTopicReady,
	Status: corev1.ConditionFalse,
}

var ignoreAllButTypeAndStatus = cmpopts.IgnoreFields(
	apis.Condition{},
	"LastTransitionTime", "Message", "Reason", "Severity")

func TestGetConditionSet(t *testing.T) {
	kc := &KafkaChannel{}
	if got, want := kc.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestChannelGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		cs        *KafkaChannelStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		cs: &KafkaChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						condReady,
					},
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &condReady,
	}, {
		name: "unknown condition",
		cs: &KafkaChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						condReady,
						condTopicNotReady,
					},
				},
			},
		},
		condQuery: apis.ConditionType("foo"),
		want:      nil,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cs.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestInitializeConditions(t *testing.T) {
	testCases := []struct {
		name string              // TestCase Name
		cs   *KafkaChannelStatus // Starting ConditionSet
		want *KafkaChannelStatus // Expected ConditionSet
	}{{
		name: "empty",
		cs:   &KafkaChannelStatus{},
		want: &KafkaChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   KafkaChannelConditionAddressable,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionChannelServiceReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionConfigReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionTopicReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}, {
		name: "one false",
		cs: &KafkaChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   KafkaChannelConditionTopicReady,
						Status: corev1.ConditionFalse,
					}},
				},
			},
		},
		want: &KafkaChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   KafkaChannelConditionAddressable,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionChannelServiceReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionConfigReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionTopicReady,
						Status: corev1.ConditionFalse,
					}},
				},
			},
		},
	}, {
		name: "one true",
		cs: &KafkaChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   KafkaChannelConditionTopicReady,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
		want: &KafkaChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   KafkaChannelConditionAddressable,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionChannelServiceReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionConfigReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   KafkaChannelConditionTopicReady,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
	}}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.cs.InitializeConditions()
			if diff := cmp.Diff(testCase.want, testCase.cs, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestChannelIsReady(t *testing.T) {
	tests := []struct {
		name                    string
		markChannelServiceReady bool
		markConfigurationReady  bool
		setAddress              bool
		markTopicReady          bool
		wantReady               bool
	}{{
		name:                    "all happy",
		markChannelServiceReady: true,
		markConfigurationReady:  true,
		setAddress:              true,
		markTopicReady:          true,
		wantReady:               true,
	}, {
		name:                    "address not set",
		markConfigurationReady:  true,
		markChannelServiceReady: true,
		setAddress:              false,
		markTopicReady:          true,
		wantReady:               false,
	}, {
		name:                    "channel service not ready",
		markConfigurationReady:  true,
		markChannelServiceReady: false,
		setAddress:              true,
		markTopicReady:          true,
		wantReady:               false,
	}, {
		name:                    "topic not ready",
		markConfigurationReady:  true,
		markChannelServiceReady: true,
		setAddress:              true,
		markTopicReady:          false,
		wantReady:               false,
	}, {
		name:                    "configuration not ready",
		markConfigurationReady:  false,
		markChannelServiceReady: true,
		setAddress:              true,
		markTopicReady:          true,
		wantReady:               false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := &KafkaChannelStatus{}
			cs.InitializeConditions()
			if test.markChannelServiceReady {
				cs.MarkChannelServiceTrue()
			} else {
				cs.MarkChannelServiceFailed("NotReadyChannelService", "testing")
			}
			if test.markConfigurationReady {
				cs.MarkConfigTrue()
			} else {
				cs.MarkConfigFailed("NotReadyConfiguration", "testing")
			}
			if test.setAddress {
				cs.SetAddress(&apis.URL{Scheme: "http", Host: "foo.bar"})
			}
			if test.markTopicReady {
				cs.MarkTopicTrue()
			} else {
				cs.MarkTopicFailed("NotReadyTopic", "testing")
			}
			got := cs.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}

func TestKafkaChannelStatus_SetAddressable(t *testing.T) {
	testCases := map[string]struct {
		url  *apis.URL
		want *KafkaChannelStatus
	}{
		"empty string": {
			want: &KafkaChannelStatus{
				ChannelableStatus: eventingduckv1.ChannelableStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							{
								Type:   KafkaChannelConditionAddressable,
								Status: corev1.ConditionFalse,
							},
							// Note that Ready is here because when the condition is marked False, duck
							// automatically sets Ready to false.
							{
								Type:   KafkaChannelConditionReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
					AddressStatus: duckv1.AddressStatus{Address: &duckv1.Addressable{}},
				},
			},
		},
		"has domain": {
			url: &apis.URL{Scheme: "http", Host: "test-domain"},
			want: &KafkaChannelStatus{
				ChannelableStatus: eventingduckv1.ChannelableStatus{
					AddressStatus: duckv1.AddressStatus{
						Address: &duckv1.Addressable{
							URL: &apis.URL{
								Scheme: "http",
								Host:   "test-domain",
							},
						},
					},
					Status: duckv1.Status{
						Conditions: []apis.Condition{{
							Type:   KafkaChannelConditionAddressable,
							Status: corev1.ConditionTrue,
						}, {
							// Ready unknown comes from other dependent conditions via MarkTrue.
							Type:   KafkaChannelConditionReady,
							Status: corev1.ConditionUnknown,
						}},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cs := &KafkaChannelStatus{}
			cs.SetAddress(tc.url)
			if diff := cmp.Diff(tc.want, cs, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestRegisterAlternateKafkaChannelConditionSet(t *testing.T) {

	cs := apis.NewLivingConditionSet(apis.ConditionReady, "hello")

	RegisterAlternateKafkaChannelConditionSet(cs)

	kc := KafkaChannel{}

	assert.Equal(t, cs, kc.GetConditionSet())
	assert.Equal(t, cs, kc.Status.GetConditionSet())
}
