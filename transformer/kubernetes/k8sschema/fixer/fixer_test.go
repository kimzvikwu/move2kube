/*
 *  Copyright IBM Corporation 2026
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package fixer

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apps "k8s.io/kubernetes/pkg/apis/apps"
	networking "k8s.io/kubernetes/pkg/apis/networking"
)

func TestDeploymentFixer(t *testing.T) {
	f := deploymentFixer{}
	if gvk := f.getGroupVersionKind(); gvk.Kind != "Deployment" || gvk.Group != "apps" {
		t.Fatalf("unexpected GVK %v", gvk)
	}

	labels := map[string]string{"app": "web"}
	d := &apps.Deployment{}
	d.Spec.Template.Labels = labels
	obj, err := f.fix(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fixed := obj.(*apps.Deployment)
	if fixed.Spec.Selector == nil || !cmp.Equal(labels, fixed.Spec.Selector.MatchLabels) {
		t.Fatalf("selector should default to the template labels: %+v", fixed.Spec.Selector)
	}

	existing := &metav1.LabelSelector{MatchLabels: map[string]string{"keep": "me"}}
	d = &apps.Deployment{}
	d.Spec.Selector = existing
	d.Spec.Template.Labels = labels
	obj, _ = f.fix(d)
	if obj.(*apps.Deployment).Spec.Selector != existing {
		t.Fatal("an existing selector should not be replaced")
	}

	if _, err := f.fix(&networking.Ingress{}); err == nil {
		t.Fatal("expected an error for a non-Deployment object")
	}
}

func TestIngressFixer(t *testing.T) {
	f := ingressFixer{}
	if gvk := f.getGroupVersionKind(); gvk.Kind != "Ingress" || gvk.Group != "networking.k8s.io" {
		t.Fatalf("unexpected GVK %v", gvk)
	}

	exact := networking.PathTypeExact
	ing := &networking.Ingress{Spec: networking.IngressSpec{Rules: []networking.IngressRule{{
		IngressRuleValue: networking.IngressRuleValue{HTTP: &networking.HTTPIngressRuleValue{Paths: []networking.HTTPIngressPath{
			{Path: "/a"},
			{Path: "/b", PathType: &exact},
		}}},
	}}}}
	obj, err := f.fix(ing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	paths := obj.(*networking.Ingress).Spec.Rules[0].HTTP.Paths
	if paths[0].PathType == nil || *paths[0].PathType != networking.PathTypePrefix {
		t.Fatalf("missing path type should default to Prefix: %+v", paths[0])
	}
	if *paths[1].PathType != networking.PathTypeExact {
		t.Fatalf("explicit path type should be kept: %+v", paths[1])
	}

	if _, err := f.fix(&apps.Deployment{}); err == nil {
		t.Fatal("expected an error for a non-Ingress object")
	}
}

func TestFix(t *testing.T) {
	t.Run("versioned deployment gets a selector", func(t *testing.T) {
		d := &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
			Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
			}},
		}
		out, ok := Fix(d).(*appsv1.Deployment)
		if !ok {
			t.Fatalf("Fix should return the original version, got %T", Fix(d))
		}
		if out.Spec.Selector == nil || out.Spec.Selector.MatchLabels["app"] != "web" {
			t.Fatalf("selector not set: %+v", out.Spec.Selector)
		}
	})

	t.Run("versioned ingress gets a path type", func(t *testing.T) {
		ing := &networkingv1.Ingress{
			TypeMeta: metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "Ingress"},
			Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{Path: "/"}},
				}},
			}}},
		}
		out, ok := Fix(ing).(*networkingv1.Ingress)
		if !ok {
			t.Fatalf("Fix should return the original version, got %T", Fix(ing))
		}
		pt := out.Spec.Rules[0].HTTP.Paths[0].PathType
		if pt == nil || *pt != networkingv1.PathTypePrefix {
			t.Fatalf("path type not defaulted: %v", pt)
		}
	})

	t.Run("other kinds are returned unchanged", func(t *testing.T) {
		svc := &corev1.Service{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"}}
		var obj runtime.Object = svc
		if Fix(obj) != obj {
			t.Fatal("objects without a fixer should be returned as is")
		}
	})
}
