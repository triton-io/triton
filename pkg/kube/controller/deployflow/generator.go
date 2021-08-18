/*
Copyright 2021 The Triton Authors.

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

package deployflow

import (
	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	internaldeploy "github.com/triton-io/triton/pkg/kube/types/deploy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func generate(idl *internaldeploy.Deploy) *kruiseappsv1alpha1.CloneSet {

	template := idl.Spec.Application.Template
	template.Labels = idl.GetCloneSetLabels()
	template.Spec.ImagePullSecrets = getImagePullSecrets()

	return &kruiseappsv1alpha1.CloneSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      idl.GetInstanceName(),
			Namespace: idl.Namespace,
			Labels:    idl.GetCloneSetLabels(),
		},
		Spec: kruiseappsv1alpha1.CloneSetSpec{
			Replicas:       idl.Spec.Application.Replicas,
			Selector:       &metav1.LabelSelector{MatchLabels: idl.GetCloneSetLabels()},
			Template:       template,
			UpdateStrategy: getDefaultStrategy(),
		},
	}
}

func getDefaultStrategy() kruiseappsv1alpha1.CloneSetUpdateStrategy {
	maxSurge := intstr.FromString("60%")
	maxUnavailable := intstr.FromInt(0)

	return kruiseappsv1alpha1.CloneSetUpdateStrategy{
		MaxSurge:       &maxSurge,
		MaxUnavailable: &maxUnavailable,
		Paused:         false,
	}
}

func getImagePullSecrets() []corev1.LocalObjectReference {
	return []corev1.LocalObjectReference{
		{Name: "proharborregcred"},
		{Name: "uatharborregcred"},
	}
}
