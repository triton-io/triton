package application

import (
	internalcloneset "github.com/triton-io/triton/pkg/kube/types/cloneset"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
)

type reply struct {
	Name              string `json:"name"`
	Namespace         string `json:"namespace"`
	AvailableReplicas int    `json:"availableReplicas"`
	Replicas          int    `json:"replicas"`
	Status            string `json:"status"`
	AppID             int    `json:"appID"`
	GroupID           int    `json:"groupID"`
	Image             string `json:"image"`
	CPU               string `json:"cpu"`
	Memory            string `json:"memory"`
	GuaranteedCPU     string `json:"guaranteedCPU,omitempty"`
	GuaranteedMemory  string `json:"guaranteedMemory,omitempty"`
	PublicNetwork     bool   `json:"publicNetwork"`
	UpdateRevision    string `json:"updateRevision"`
}

func setApplicationReply(cs *kruiseappsv1alpha1.CloneSet) *reply {
	ics := internalcloneset.FromCloneSet(cs)

	return &reply{
		Name:              ics.GetName(),
		Namespace:         ics.GetNamespace(),
		AvailableReplicas: int(ics.Status.UpdatedReadyReplicas),
		Replicas:          int(*ics.Spec.Replicas),
		Status:            ics.GetPhase(),
		AppID:             ics.GetAppID(),
		GroupID:           ics.GetGroupID(),
		Image:             ics.GetAppImage(),
		CPU:               ics.GetAppCPU(),
		Memory:            ics.GetAppMemory(),
		GuaranteedCPU:     ics.GetAppRequestCPU(),
		GuaranteedMemory:  ics.GetAppRequestMemory(),
		UpdateRevision:    ics.GetUpdateRevision(),
	}
}
