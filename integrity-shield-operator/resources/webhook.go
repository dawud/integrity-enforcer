//
// Copyright 2020 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package resources

import (
	"fmt"

	apiv1alpha1 "github.com/IBM/integrity-enforcer/integrity-shield-operator/api/v1alpha1"
	admv1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
)

//service
func BuildServiceForIShield(cr *apiv1alpha1.IntegrityShield) *corev1.Service {
	var targetport intstr.IntOrString
	targetport.Type = intstr.String
	targetport.StrVal = "ac-api"
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.GetWebhookServiceName(),
			Namespace: cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       443,
					TargetPort: targetport, //"ac-api"
				},
			},
			Selector: cr.Spec.SelectorLabels,
		},
	}
	return svc
}

//webhook configuration
func BuildMutatingWebhookConfigurationForIShield(cr *apiv1alpha1.IntegrityShield) *admv1.MutatingWebhookConfiguration {

	namespaced := admv1.NamespacedScope
	cluster := admv1.ClusterScope

	namespacedRule := cr.Spec.WebhookNamespacedResource
	namespacedRule.Scope = &namespaced

	clusterRule := cr.Spec.WebhookClusterResource
	clusterRule.Scope = &cluster

	var path *string
	mutate := "/mutate"
	path = &mutate

	var empty []byte

	sideEffect := admv1.SideEffectClassNone
	timeoutSeconds := int32(apiv1alpha1.DefaultIShieldWebhookTimeout)

	wc := &admv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.GetWebhookConfigName(),
			Namespace: cr.Namespace,
		},
		Webhooks: []admv1.MutatingWebhook{
			{
				Name: fmt.Sprintf("ac-server.%s.svc", cr.Namespace),
				ClientConfig: admv1.WebhookClientConfig{
					Service: &admv1.ServiceReference{
						Name:      cr.GetWebhookServiceName(),
						Namespace: cr.Namespace,
						Path:      path, //"/mutate"
					},
					CABundle: empty,
				},
				Rules: []admv1.RuleWithOperations{
					{
						Operations: []admv1.OperationType{
							admv1.Create, admv1.Delete, admv1.Update,
						},
						Rule: namespacedRule,
					},
					{
						Operations: []admv1.OperationType{
							admv1.Create, admv1.Delete, admv1.Update,
						},
						Rule: clusterRule,
					},
				},
				SideEffects:    &sideEffect,
				TimeoutSeconds: &timeoutSeconds,
			},
		},
	}
	return wc
}
