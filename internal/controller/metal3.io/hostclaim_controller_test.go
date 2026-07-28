/*
Copyright 2025 The Metal3 Authors.

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

package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	metal3api "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/metal3-io/baremetal-operator/internal/testutil"
	"github.com/metal3-io/baremetal-operator/pkg/hostclaim"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type MockHostManager struct {
	countSetFinalizer            int
	countUnsetFinalizer          int
	countSetConditionHostToFalse int
	countSetConditionHostToTrue  int
	countAssociate               int
	countDelete                  int
	countUpdate                  int
	isProvisioned                bool
	isAssociated                 bool
	associateError               error
	updateError                  error
	deleteError                  error
	conditions                   map[string]mockCondition
}

type mockCondition struct {
	reason  string
	message string
	ok      bool
}

func (m *MockHostManager) SetFinalizer()       { m.countSetFinalizer++ }
func (m *MockHostManager) UnsetFinalizer()     { m.countUnsetFinalizer++ }
func (m *MockHostManager) IsProvisioned() bool { return m.isProvisioned }
func (m *MockHostManager) IsAssociated() bool  { return m.isAssociated }
func (m *MockHostManager) Associate(ctx context.Context) error {
	m.countAssociate++
	return m.associateError
}
func (m *MockHostManager) Delete(ctx context.Context) error {
	m.countDelete++
	return m.deleteError
}
func (m *MockHostManager) Update(ctx context.Context) error {
	m.countUpdate++
	return m.updateError
}
func (m *MockHostManager) SetConditionHostToFalse(t string, reason string, msg string) {
	if m.conditions == nil {
		m.conditions = map[string]mockCondition{}
	}
	m.conditions[t] = mockCondition{reason: reason, message: msg, ok: false}
}

func (m *MockHostManager) SetConditionHostToTrue(t string, reason string) {
	if m.conditions == nil {
		m.conditions = map[string]mockCondition{}
	}
	m.conditions[t] = mockCondition{reason: reason, ok: true}
}

// setupSchemes configures schemes.
func setupScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		panic(err)
	}

	if err := metal3api.AddToScheme(scheme); err != nil {
		panic(err)
	}

	return scheme
}

const (
	baremetalName      = "bmh"
	baremetalNamespace = "ns"
)

var (
	defaultConsumerRef = corev1.ObjectReference{
		Name:       HostclaimName,
		Namespace:  HostclaimNamespace,
		Kind:       "HostClaim",
		APIVersion: "metal3.io/v1alpha1",
	}
	errDefault        = errors.New("default")
	delay             = 30 * time.Second
	errDefaultRequeue = hostclaim.RequeueAfterError{RequeueAfter: delay}
)

type testCaseHostClaimReconcile struct {
	HostClaim            *metal3api.HostClaim
	HostManager          *MockHostManager
	DeleteRequested      bool
	ExpectError          bool
	ExpectRequeue        bool
	AssociateCalled      bool
	UpdateCalled         bool
	DeleteCalled         bool
	SetFinalizerCalled   bool
	UnsetFinalizerCalled bool
	AssociateError       error
	UpdateError          error
	DeleteError          error
}

var _ = Describe("Test HostClaim Controller",
	func() {
		DescribeTable("Test Reconcile",
			func(tc testCaseHostClaimReconcile) {
				ctx := context.TODO()
				objs := []client.Object{tc.HostClaim}
				key := types.NamespacedName{Name: HostclaimName, Namespace: HostclaimNamespace}
				scheme := setupScheme()
				builder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithStatusSubresource(tc.HostClaim)
				fakeClient := builder.Build()
				if tc.DeleteRequested {
					hostClaim := &metal3api.HostClaim{}
					err := fakeClient.Get(ctx, key, hostClaim)
					Expect(err).NotTo(HaveOccurred())
					err = fakeClient.Delete(ctx, hostClaim)
					Expect(err).NotTo(HaveOccurred())
				}
				namespacedName := types.NamespacedName{
					Namespace: HostclaimNamespace,
					Name:      HostclaimName,
				}
				req := ctrl.Request{NamespacedName: namespacedName}
				hostclaimCtrl := HostClaimReconciler{
					Client: fakeClient, Scheme: scheme, Log: GinkgoLogr, APIReader: fakeClient,
					NewHostClaimManager: func(_ client.Client, _ logr.Logger, _ *metal3api.HostClaim, _ client.Reader,
					) hostclaim.ManagerInterface {
						return tc.HostManager
					}}
				result, err := hostclaimCtrl.Reconcile(ctx, req)
				if tc.ExpectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
					if tc.ExpectRequeue {
						Expect(result.RequeueAfter).NotTo(BeZero())
					} else {
						Expect(result.RequeueAfter).To(BeZero())
					}
				}
				if tc.AssociateCalled {
					Expect(tc.HostManager.countAssociate).To(Equal(1))
				} else {
					Expect(tc.HostManager.countAssociate).To(BeZero())
				}
				if tc.DeleteCalled {
					Expect(tc.HostManager.countDelete).To(Equal(1))
				} else {
					Expect(tc.HostManager.countDelete).To(BeZero())
				}
				if tc.UpdateCalled {
					Expect(tc.HostManager.countUpdate).To(Equal(1))
				} else {
					Expect(tc.HostManager.countUpdate).To(BeZero())
				}
				if tc.UnsetFinalizerCalled {
					Expect(tc.HostManager.countUnsetFinalizer).To(Equal(1))
				} else {
					Expect(tc.HostManager.countUnsetFinalizer).To(BeZero())
				}
				if tc.SetFinalizerCalled {
					Expect(tc.HostManager.countSetFinalizer).To(Equal(1))
				} else {
					Expect(tc.HostManager.countSetFinalizer).To(BeZero())
				}

			},
			Entry("No HostClaim", testCaseHostClaimReconcile{
				HostClaim:   NewHostclaim("Another").Build(),
				HostManager: &MockHostManager{},
			}),
			Entry("Not associated", testCaseHostClaimReconcile{
				HostClaim:          NewHostclaim(HostclaimName).Build(),
				HostManager:        &MockHostManager{},
				AssociateCalled:    true,
				UpdateCalled:       true,
				SetFinalizerCalled: true,
			}),
			Entry("Not associated - associate errs", testCaseHostClaimReconcile{
				HostClaim:          NewHostclaim(HostclaimName).Build(),
				HostManager:        &MockHostManager{associateError: errDefault},
				AssociateCalled:    true,
				ExpectError:        true,
				SetFinalizerCalled: true,
			}),
			Entry("Not associated - associate requeue", testCaseHostClaimReconcile{
				HostClaim:          NewHostclaim(HostclaimName).Build(),
				HostManager:        &MockHostManager{associateError: errDefaultRequeue},
				AssociateCalled:    true,
				ExpectRequeue:      true,
				SetFinalizerCalled: true,
			}),
			Entry("Not associated - update errs", testCaseHostClaimReconcile{
				HostClaim:          NewHostclaim(HostclaimName).Build(),
				HostManager:        &MockHostManager{updateError: errDefault},
				AssociateCalled:    true,
				UpdateCalled:       true,
				ExpectError:        true,
				SetFinalizerCalled: true,
			}),
			Entry("Associated", testCaseHostClaimReconcile{
				HostClaim:          NewHostclaim(HostclaimName).Build(),
				HostManager:        &MockHostManager{isAssociated: true},
				UpdateCalled:       true,
				SetFinalizerCalled: true,
			}),
			Entry("Associated - error", testCaseHostClaimReconcile{
				HostClaim:          NewHostclaim(HostclaimName).Build(),
				HostManager:        &MockHostManager{isAssociated: true, updateError: errDefault},
				ExpectError:        true,
				UpdateCalled:       true,
				SetFinalizerCalled: true,
			}),
			Entry("Provisioned", testCaseHostClaimReconcile{
				HostClaim:          NewHostclaim(HostclaimName).Build(),
				HostManager:        &MockHostManager{isProvisioned: true},
				UpdateCalled:       true,
				SetFinalizerCalled: true,
			}),
			Entry("Provisioned - error", testCaseHostClaimReconcile{
				HostClaim:          NewHostclaim(HostclaimName).Build(),
				HostManager:        &MockHostManager{isProvisioned: true, updateError: errDefault},
				UpdateCalled:       true,
				SetFinalizerCalled: true,
				ExpectError:        true,
			}),
			Entry("Delete", testCaseHostClaimReconcile{
				HostClaim:            NewHostclaim(HostclaimName).SetFinalizer([]string{metal3api.HostClaimFinalizer}).Build(),
				HostManager:          &MockHostManager{},
				DeleteRequested:      true,
				DeleteCalled:         true,
				UnsetFinalizerCalled: true,
			}),
			Entry("Delete - requeue requested", testCaseHostClaimReconcile{
				HostClaim:       NewHostclaim(HostclaimName).SetFinalizer([]string{metal3api.HostClaimFinalizer}).Build(),
				HostManager:     &MockHostManager{deleteError: errDefaultRequeue},
				DeleteRequested: true,
				DeleteCalled:    true,
				ExpectRequeue:   true,
			}),
		)

		It("Test BareMetalHostToHostClaims (with consumer ref)", func() {
			bmh := NewBaremetalhost(baremetalName, baremetalNamespace, metal3api.StateAvailable).SetConsumerRef(defaultConsumerRef).Build()
			reqList := BareMetalHostToHostClaims(context.TODO(), bmh)
			Expect(reqList).To(HaveLen(1))
			Expect(reqList[0].Name).To(Equal(HostclaimName))
			Expect(reqList[0].Namespace).To(Equal(HostclaimNamespace))
		})

		It("Test BareMetalHostToHostClaims (no consumer ref)", func() {
			reqList := BareMetalHostToHostClaims(context.TODO(), NewBaremetalhost(baremetalName, baremetalNamespace, metal3api.StateAvailable).Build())
			Expect(reqList).To(BeEmpty())
		})
	},
)

func TestManagers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manager Suite")
}
