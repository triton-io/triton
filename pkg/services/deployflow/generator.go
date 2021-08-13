package deployflow

import (
	"encoding/json"
	"fmt"

	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	"github.com/triton-io/triton/pkg/kube/types/workload"
	"github.com/triton-io/triton/pkg/setting"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type generator struct {
	appID             int
	groupID           int
	replicas          int32
	namespace         string
	appName           string
	instanceName      string
	action            string
	applicationSpec   *tritonappsv1alpha1.ApplicationSpec
	updateStrategy    *tritonappsv1alpha1.DeployUpdateStrategy
	nonUpdateStrategy *tritonappsv1alpha1.DeployNonUpdateStrategy
}

func (g *generator) generate() *tritonappsv1alpha1.DeployFlow {
	var lastApplied []byte

	if g.applicationSpec != nil {
		lastApplied, _ = json.Marshal(g.applicationSpec)
	}

	return &tritonappsv1alpha1.DeployFlow{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", g.instanceName),
			Namespace:    g.namespace,
			Labels:       g.getDefaultLabels(),
			Annotations:  g.getLastAppliedAnnotations(lastApplied),
		},
		Spec: tritonappsv1alpha1.DeployFlowSpec{
			Application: &tritonappsv1alpha1.ApplicationSpec{
				Selector:     &metav1.LabelSelector{MatchLabels: g.getDefaultLabels()},
				AppID:        g.applicationSpec.AppID,
				GroupID:      g.applicationSpec.GroupID,
				AppName:      g.applicationSpec.AppName,
				InstanceName: g.applicationSpec.InstanceName,
				Template:     g.applicationSpec.Template,
				Replicas:     &g.replicas,
			},

			Action:            g.action,
			UpdateStrategy:    g.updateStrategy,
			NonUpdateStrategy: g.nonUpdateStrategy,
		},
	}
}

func (g *generator) getDefaultLabels() labels.Set {
	return workload.GetDefaultLabels(g.appName, g.instanceName, g.appID, g.groupID)
}

func (g *generator) getLastAppliedAnnotations(lastApplied []byte) labels.Set {
	return labels.Set{
		setting.LastAppliedLabel: string(lastApplied),
	}
}
