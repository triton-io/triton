package deploy

import tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"

type ConditionFunc func(d *tritonappsv1alpha1.DeployFlow) (bool, error)
type ConditionFuncs []ConditionFunc

func getCancelConditions() ConditionFuncs {
	return ConditionFuncs{
		func(d *tritonappsv1alpha1.DeployFlow) (bool, error) {
			return d.Status.Phase == tritonappsv1alpha1.Canceled, nil
		},
	}
}

func getPauseConditions() ConditionFuncs {
	return ConditionFuncs{
		func(d *tritonappsv1alpha1.DeployFlow) (bool, error) {
			return d.Status.Paused, nil
		},
	}
}

func getResumeConditions() ConditionFuncs {
	return ConditionFuncs{
		func(d *tritonappsv1alpha1.DeployFlow) (bool, error) {
			return !d.Status.Paused, nil
		},
	}
}
