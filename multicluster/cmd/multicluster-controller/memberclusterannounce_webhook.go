/*
Copyright 2021 Antrea Authors.

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

package main

import (
	multiclusterv1alpha1 "antrea.io/antrea/multicluster/apis/multicluster/v1alpha1"
	"context"
	"fmt"
	serviceaccount "k8s.io/apiserver/pkg/authentication/serviceaccount"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var memberclusterannouncelog = logf.Log.WithName("memberclusterannounce-resource")

//+kubebuilder:webhook:path=/validate-multicluster-crd-antrea-io-v1alpha1-memberclusterannounce,mutating=false,failurePolicy=fail,sideEffects=None,groups=multicluster.crd.antrea.io,resources=memberclusterannounces,verbs=create;update,versions=v1alpha1,name=vmemberclusterannounce.kb.io,admissionReviewVersions={v1,v1beta1}

// member cluster announce validator
type memberClusterAnnounceValidator struct {
	Client    client.Client
	decoder   *admission.Decoder
	namespace string
}

// Handle handles admission requests.
func (v *memberClusterAnnounceValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	memberClusterAnnounce := &multiclusterv1alpha1.MemberClusterAnnounce{}
	e := v.decoder.Decode(req, memberClusterAnnounce)
	if e != nil {
		return admission.Errored(http.StatusBadRequest, e)
	}

	ui := req.UserInfo
	_, saName, err := serviceaccount.SplitUsername(ui.Username)
	if err != nil {
		memberclusterannouncelog.Error(err, "Error getting service account name", "request", req)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// read the cluster set info
	clusterSetList := &multiclusterv1alpha1.ClusterSetList{}
	if err := v.Client.List(context.TODO(), clusterSetList, client.InNamespace(v.namespace)); err != nil {
		memberclusterannouncelog.Error(err, "Error reading clusterSet", "namespace", v.namespace)
		return admission.Errored(http.StatusPreconditionFailed, err)
	}

	if len(clusterSetList.Items) != 1 {
		memberclusterannouncelog.Info("Invalid clusterSet config", "namespace", v.namespace)
		return admission.Errored(http.StatusPreconditionFailed, fmt.Errorf("Invalid clusterSet config"))
	}

	clusterSet := clusterSetList.Items[0]
	for _, member := range clusterSet.Spec.Members {
		if clusterSet.Name == memberClusterAnnounce.ClusterSetID && member.ClusterID == memberClusterAnnounce.ClusterID {
			// validate the service account used is correct
			if member.ServiceAccount == saName {
				return admission.Allowed("")
			} else {
				memberclusterannouncelog.Info("Does not have permission to write member announce", "member", member.ClusterID)
				return admission.Denied("Member does not have permissions")
			}
		}
	}

	memberclusterannouncelog.Info("Not defined in clusterset", "member", memberClusterAnnounce.ClusterID, "clusterset", clusterSet.Name)
	return admission.Denied("Unknown member")
}

func (v *memberClusterAnnounceValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
