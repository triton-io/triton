package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/triton-io/triton/pkg/kube/watcher"
	"github.com/triton-io/triton/pkg/log"
	"github.com/triton-io/triton/pkg/services/deployflow"
	"github.com/triton-io/triton/pkg/setting"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	terrors "github.com/triton-io/triton/pkg/errors"
	kubeclient "github.com/triton-io/triton/pkg/kube/client"
	internaldeploy "github.com/triton-io/triton/pkg/kube/types/deploy"
	pb "github.com/triton-io/triton/pkg/protos/deployflow"
)

type Service struct {
	pb.UnimplementedDeployFlowServer
}

func (s *Service) Get(_ context.Context, in *pb.DeployMetaRequest) (*pb.DeployReply, error) {
	d, err := getDeploy(in.Deploy.Namespace, in.Deploy.Name)
	return setDeployReply(d), err
}

func (s *Service) Gets(_ context.Context, in *pb.DeploysRequest) (*pb.DeploysReply, error) {
	ds, err := getDeploys(grpcFilterToFetcherFilter(in.Filter))
	if err != nil {
		return nil, err
	}

	return setDeploysReply(ds), nil
}

func (s *Service) Cancel(in *pb.DeployMetaRequest, stream pb.DeployFlow_CancelServer) error {
	strategyBytes := []byte(fmt.Sprintf(`{"canceled":%t}`, true))
	return patchAndWait(in.Deploy.Namespace, in.Deploy.Name, strategyBytes, stream, getCancelConditions()...)
}

func (s *Service) Pause(in *pb.DeployMetaRequest, stream pb.DeployFlow_PauseServer) error {
	strategyBytes := []byte(fmt.Sprintf(`{"paused":%t}`, true))
	return patchAndWait(in.Deploy.Namespace, in.Deploy.Name, strategyBytes, stream, getPauseConditions()...)
}

func (s *Service) Resume(in *pb.DeployMetaRequest, stream pb.DeployFlow_ResumeServer) error {
	strategyBytes := []byte(fmt.Sprintf(`{"paused":%t}`, false))
	return patchAndWait(in.Deploy.Namespace, in.Deploy.Name, strategyBytes, stream, getResumeConditions()...)
}

func (s *Service) Continue(_ context.Context, in *pb.ContinueRequest) (*pb.DeployReply, error) {
	if in.Target == nil {
		return nil, status.Error(codes.InvalidArgument, "empty target")
	}

	strategyBytes, err := json.Marshal(in.Target)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "bad target")
	}

	return patchAndSetDeployReply(in.Deploy.Namespace, in.Deploy.Name, strategyBytes)
}

func (s *Service) Next(_ context.Context, in *pb.NextRequest) (*pb.DeployReply, error) {
	deploy, err := getDeploy(in.Deploy.Namespace, in.Deploy.Name)
	if err != nil {
		return nil, err
	}

	idf := internaldeploy.FromDeploy(deploy)
	if idf.Finished() {
		return nil, status.Error(codes.FailedPrecondition, "deploy is finished")
	}
	if !idf.StateSatisfied() {
		return nil, status.Error(codes.FailedPrecondition, "last batch or stage is not finished yet")
	}

	nextBatch, nextStage := idf.NextBatchAndPhase()
	strategyBytes := []byte(fmt.Sprintf(`{"batches":%d,"stage":"%s"}`, nextBatch, nextStage))

	return patchAndSetDeployReply(in.Deploy.Namespace, in.Deploy.Name, strategyBytes)
}

func (s *Service) Delete(_ context.Context, in *pb.DeployMetaRequest) (*pb.EmptyReply, error) {
	logger := log.WithFields(logrus.Fields{
		"context":   "deploy",
		"namespace": in.Deploy.Namespace,
		"name":      in.Deploy.Name,
	})
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	err := deployflow.RemoveDeploy(in.Deploy.Namespace, in.Deploy.Name, cl, logger)
	if err != nil {
		if terrors.IsNotFound(err) {
			return &pb.EmptyReply{}, nil
		} else if terrors.IsConflict(err) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.EmptyReply{}, nil
}

func (s *Service) Watch(in *pb.WatchRequest, stream pb.DeployFlow_WatchServer) error {
	logger := log.WithFields(logrus.Fields{
		"context":   "deploy",
		"method":    "Watch",
		"namespace": in.Deploy.Namespace,
		"name":      in.Deploy.Name,
	})

	return watcher.WatchObject(
		stream.Context(),
		&tritonappsv1alpha1.DeployFlow{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: in.Deploy.Namespace,
				Name:      in.Deploy.Name,
			},
		},
		kubeclient.NewManager(),
		logger,
		func(obj runtime.Object) (bool, error) {
			d, ok := obj.(*tritonappsv1alpha1.DeployFlow)
			if !ok {
				return false, fmt.Errorf("object is not a DeployFlow")
			}

			err := stream.Send(setDeployReply(d))
			if err != nil {
				logger.WithError(err).Error("Failed to send to stream")
				return false, err
			}

			idf := internaldeploy.FromDeploy(d)

			// stop when it is finished
			if idf.Finished() {
				logger.Info("Deploy is finished, stop watching")
				return true, nil
			}

			if in.Target != nil {
				// stop when target state is met
				if internaldeploy.StateSatisfied(idf, tritonappsv1alpha1.BatchPhase(in.Target.Stage), int(in.Target.Batches)) {
					logger.Info("Target state reached, stop watching")
					return true, nil
				}
			} else {
				// stop when target is empty but desired state is met
				if idf.StateSatisfied() {
					logger.Info("Desired state reached, stop watching")
					return true, nil
				}
			}

			return false, nil
		},
	)
}

func (s *Service) ListAndWatch(in *pb.DeploysRequest, stream pb.DeployFlow_ListAndWatchServer) error {
	logger := log.WithFields(logrus.Fields{
		"context": "deploy",
		"method":  "ListAndWatch",
	})

	// list all Deploys
	filter := grpcFilterToFetcherFilter(in.Filter)
	ds, err := getDeploys(filter)
	if err != nil {
		return err
	}

	err = stream.Send(setDeploysReply(ds))
	if err != nil {
		logger.WithError(err).Error("Failed to send to stream")
		return err
	}

	// start watching
	return watcher.WatchManagedObjects(
		stream.Context(),
		&tritonappsv1alpha1.DeployFlow{TypeMeta: metav1.TypeMeta{Kind: setting.TypeDeploy}},
		kubeclient.NewManager(),
		func(obj runtime.Object) error {
			d, ok := obj.(*tritonappsv1alpha1.DeployFlow)
			if !ok {
				return fmt.Errorf("object is not a DeployFlow")
			}

			if !matchFilter(filter, d) {
				return nil
			}

			err = stream.Send(setDeploysReply([]*tritonappsv1alpha1.DeployFlow{d}))
			if err != nil {
				logger.WithError(err).Error("Failed to send to stream")
				return err
			}

			return nil
		},
	)
}

func (s *Service) WatchFixedInterval(in *pb.WatchRequest, stream pb.DeployFlow_WatchServer) error {
	var origin, updated *tritonappsv1alpha1.DeployFlow
	var err error

	for {
		origin = updated
		updated, err = getDeploy(in.Deploy.Namespace, in.Deploy.Name)
		if err != nil {
			return err
		}
		if origin == nil || !reflect.DeepEqual(origin.Status, updated.Status) {
			err = stream.Send(setDeployReply(updated))
			if err != nil {
				return err
			}
		}

		idf := internaldeploy.FromDeploy(updated)

		// stop when it is finished
		if idf.Finished() {
			return nil
		}

		if in.Target != nil {
			// stop when target state is met
			if internaldeploy.StateSatisfied(idf, tritonappsv1alpha1.BatchPhase(in.Target.Stage), int(in.Target.Batches)) {
				return nil
			}
		} else {
			// stop when target is empty but desired state is met
			if idf.StateSatisfied() {
				return nil
			}
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func patchDeploy(ns, name string, strategyBytes []byte) (*tritonappsv1alpha1.DeployFlow, error) {
	d, err := getDeploy(ns, name)
	if err != nil {
		return nil, err
	}
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()
	cr := mgr.GetAPIReader()

	if internaldeploy.FromDeploy(d).Finished() {
		return nil, status.Error(codes.FailedPrecondition, "changes on a finished deploy is not allowed")
	}

	d, err = deployflow.PatchDeployStrategy(ns, name, d.Spec.Action, cr, cl, strategyBytes)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "deploy not found")
		}
		log.WithError(err).Error("failed to patch deploy")
		return nil, status.Error(codes.Internal, "failed to patch deploy")
	}

	return d, nil
}

func waitConditions(ns, name string, sender StreamSender, conditions ...ConditionFunc) error {
	return wait.PollImmediateUntil(200*time.Millisecond, func() (bool, error) {
		d, err := getDeploy(ns, name)

		if err != nil {
			return false, err
		}
		err = sender.Send(setDeployReply(d))
		if err != nil {
			return false, err
		}

		if len(conditions) == 0 || d.Status.Finished {
			return true, nil
		}

		for _, c := range conditions {
			done, err := c(d)
			if err != nil || done {
				return done, err
			}
		}

		return false, nil

	}, sender.Context().Done())
}

func patchAndWait(ns, name string, strategyBytes []byte, sender StreamSender, conditions ...ConditionFunc) error {
	_, err := patchDeploy(ns, name, strategyBytes)
	if err != nil {
		return err
	}
	return waitConditions(ns, name, sender, conditions...)
}

func patchAndSetDeployReply(ns, name string, strategyBytes []byte) (*pb.DeployReply, error) {
	d, err := patchDeploy(ns, name, strategyBytes)
	return setDeployReply(d), err
}
