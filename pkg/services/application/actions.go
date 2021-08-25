package application

import (
	"context"
	"fmt"
	"time"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/sirupsen/logrus"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	"github.com/triton-io/triton/pkg/setting"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	terrors "github.com/triton-io/triton/pkg/errors"
	kubeclient "github.com/triton-io/triton/pkg/kube/client"
)

func RemoveApplication(ns, name string, logger *logrus.Entry) error {
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	cs, found, err := fetcher.GetCloneSetInCache(ns, name, cl)
	if err != nil {
		logger.WithError(err).Error("failed to get application")
		return err
	} else if !found {
		return terrors.NewNotFound(fmt.Sprintf("application %s not found", name))
	}

	owner := metav1.GetControllerOf(cs)
	if owner != nil && owner.Kind == setting.TypeDeploy {
		return terrors.NewConflict(fmt.Sprintf("deleting an application owned by an in-progress deploy %s is not allowed", owner.Name), nil)
	}

	logger.Info("Start to delete application")
	if err := cl.Delete(context.TODO(), &kruiseappsv1alpha1.CloneSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.WithError(err).Error("failed to delete deploy")
			return err
		}
	}

	// wait till application is deleted from local cache
	err = wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		_, found, err := fetcher.GetCloneSetInCache(ns, name, cl)
		if err != nil {
			return false, err
		} else if found {
			return false, nil
		}
		return true, nil
	})

	logger.Info("Finished to delete deploy")

	return nil
}
