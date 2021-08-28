package watcher

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/triton-io/triton/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type ActionFunc func(obj runtime.Object) (done bool, err error)
type InfiniteActionFunc func(obj runtime.Object) error
type ActionFuncs []ActionFunc

type Watcher interface {
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
}

// WatchSingleObject watches changes of the given object, and apply the registered actions for every change
func WatchSingleObject(ctx context.Context, watchClient Watcher, obj metav1.Object, actions ...ActionFunc) error {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", obj.GetName()).String()
	watcher, err := watchClient.Watch(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return err
	}
	defer watcher.Stop()
	return Watch(ctx, watcher, actions...)
}

func WatchObjects(
	ctx context.Context, watchClient Watcher,
	labelSelector, fieldSelector string,
	actions ...ActionFunc,
) error {
	watcher, err := watchClient.Watch(ctx, metav1.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector})
	if err != nil {
		return err
	}
	defer watcher.Stop()
	return Watch(ctx, watcher, actions...)
}

func Watch(ctx context.Context, watcher watch.Interface, actions ...ActionFunc) error {
	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok || event.Type == watch.Error {
				log.WithField("channelClosed", !ok).WithField("eventType", event.Type).Debugf("Exit the watch loop.")
				return nil
			}
			for _, action := range actions {
				if done, err := action(event.Object); err != nil {
					return err
				} else if done {
					log.Debug("Exit the watch loop since task is done.")
					return nil
				}
			}

		case <-ctx.Done():
			log.Debug("Exit the watch loop since context is done.")
			return nil
		}
	}
}

func WatchObject(ctx context.Context, obj runtime.Object, mgr manager.Manager, logger *logrus.Entry, action ActionFunc) error {
	meta, ok := obj.(metav1.Object)
	if !ok {
		logger.Error("metadata is missing")
		return fmt.Errorf("metadata is missing")
	}

	watchCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	r := &ActionReconciler{
		Action: action,
		Type:   obj,
		Done:   cancelFunc,
		Logger: logger,
	}
	c, err := controller.NewUnmanaged(meta.GetName(), mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: obj},
		&EnqueueRequestForObject{},
		singleObjectPredicate{Namespace: meta.GetNamespace(), Name: meta.GetName()},
	)

	if err != nil {
		return err
	}

	go func() {
		if err := c.Start(watchCtx.Done()); err != nil {
			logger.WithError(err).Error("Failed to start watcher")
		}

		cancelFunc()
	}()

	<-watchCtx.Done()

	return nil
}

func WatchManagedObjects(ctx context.Context, obj runtime.Object, mgr manager.Manager, action InfiniteActionFunc) error {
	watchCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	r := reconcile.Func(func(req reconcile.Request) (reconcile.Result, error) {
		if err := mgr.GetClient().Get(context.TODO(), req.NamespacedName, obj); err != nil {
			return reconcile.Result{}, client.IgnoreNotFound(err)
		}
		err := action(obj)

		return reconcile.Result{}, err
	})

	kind := obj.GetObjectKind().GroupVersionKind().Kind
	logger := log.WithField("kind", kind)
	c, err := controller.NewUnmanaged(kind, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: obj},
		&EnqueueRequestForObject{},
		predicate.ResourceVersionChangedPredicate{},
	)

	if err != nil {
		return err
	}

	go func() {
		logger.Info("Start to watch object")
		err = c.Start(watchCtx.Done())
		if err != nil {
			log.WithError(err).Error("Failed to start watcher")
		}
		logger.Info("Watcher stopped")

		cancelFunc()
	}()

	<-watchCtx.Done()

	return err
}
