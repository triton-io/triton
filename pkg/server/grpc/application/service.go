package application

import (
	"context"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/sirupsen/logrus"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	"github.com/triton-io/triton/pkg/log"
	"github.com/triton-io/triton/pkg/setting"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/intstr"

	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	terrors "github.com/triton-io/triton/pkg/errors"
	kubeclient "github.com/triton-io/triton/pkg/kube/client"
	internalcloneset "github.com/triton-io/triton/pkg/kube/types/cloneset"
	pb "github.com/triton-io/triton/pkg/protos/application"
	applicationservice "github.com/triton-io/triton/pkg/services/application"
	"github.com/triton-io/triton/pkg/services/deployflow"
)

type Service struct {
	pb.UnimplementedApplicationServer
}

func (s *Service) Gets(_ context.Context, in *pb.GetsRequest) (*pb.InstancesReply, error) {
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	css, err := fetcher.GetCloneSetsInCache(in.Filter.Namespace, cl)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.InstancesReply{Instances: setInstancesReply(css)}, nil
}

func (s *Service) Get(_ context.Context, in *pb.InstanceMetaRequest) (*pb.InstanceReply, error) {
	cs, err := getInstance(in.Instance.Namespace, in.Instance.Name)
	if err != nil {
		return nil, err
	}

	return &pb.InstanceReply{Instance: setInstanceReply(cs)}, nil
}

func (s *Service) Delete(_ context.Context, in *pb.InstanceMetaRequest) (*pb.EmptyReply, error) {
	logger := log.WithFields(logrus.Fields{
		"context":      "application",
		"namespace":    in.Instance.Namespace,
		"instanceName": in.Instance.Name,
	})

	err := applicationservice.RemoveApplication(in.Instance.Namespace, in.Instance.Name, logger)
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

func (s *Service) Restart(_ context.Context, in *pb.RestartRequest) (*pb.RestartReply, error) {
	logger := log.WithFields(logrus.Fields{
		"context":      "application",
		"namespace":    in.Instance.Namespace,
		"instanceName": in.Instance.Name,
	})
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	cs, found, err := fetcher.GetCloneSetInCache(in.Instance.Namespace, in.Instance.Name, cl)
	if err != nil {
		logger.WithError(err).Error("failed to fetch cloneSet")
		return nil, err
	} else if !found {
		return nil, terrors.NewNotFound("cloneSet not found")
	}

	ics := internalcloneset.FromCloneSet(cs)

	size := intstr.Parse(in.Strategy.BatchSize)
	strategy := &tritonappsv1alpha1.DeployNonUpdateStrategy{
		BaseStrategy: tritonappsv1alpha1.BaseStrategy{
			BatchSize:            &size,
			Batches:              int(in.Strategy.Batches),
			BatchIntervalSeconds: in.Strategy.BatchIntervalSeconds,
			Mode:                 tritonappsv1alpha1.DeployMode(in.Strategy.Mode),
		},
		PodsToDelete: in.Strategy.PodsToDelete,
	}

	applicationSpec := tritonappsv1alpha1.ApplicationSpec{
		AppID:        ics.GetAppID(),
		GroupID:      ics.GetGroupID(),
		Replicas:     ics.Spec.Replicas,
		AppName:      ics.GetAppName(),
		Template:     ics.Spec.Template,
		InstanceName: ics.Name,
	}

	req := &deployflow.DeployNonUpdateRequest{
		Action:            setting.Restart,
		ApplicationSpec:   &applicationSpec,
		NonUpdateStrategy: strategy,
	}
	updated, err := deployflow.CreateNonUpdateDeploy(req, ics.Namespace, cl, logger)
	if err != nil {
		if terrors.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "application not found")
		} else if terrors.IsConflict(err) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.RestartReply{DeployName: updated.Name}, nil
}

func (s *Service) Scale(_ context.Context, in *pb.ScaleRequest) (*pb.ScaleReply, error) {
	logger := log.WithFields(logrus.Fields{
		"context":      "application",
		"namespace":    in.Instance.Namespace,
		"instanceName": in.Instance.Name,
	})
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()
	cs, found, err := fetcher.GetCloneSetInCache(in.Instance.Namespace, in.Instance.Name, cl)
	if err != nil {
		logger.WithError(err).Error("failed to fetch cloneSet")
		return nil, err
	} else if !found {
		return nil, terrors.NewNotFound("cloneSet not found")
	}

	ics := internalcloneset.FromCloneSet(cs)

	size := intstr.Parse(in.Strategy.BatchSize)
	strategy := &tritonappsv1alpha1.DeployNonUpdateStrategy{
		BaseStrategy: tritonappsv1alpha1.BaseStrategy{
			BatchSize:            &size,
			Batches:              int(in.Strategy.Batches),
			BatchIntervalSeconds: in.Strategy.BatchIntervalSeconds,
			Mode:                 tritonappsv1alpha1.DeployMode(in.Strategy.Mode),
		},
		PodsToDelete: in.Strategy.PodsToDelete,
	}
	applicationSpec := tritonappsv1alpha1.ApplicationSpec{
		AppID:        ics.GetAppID(),
		GroupID:      ics.GetGroupID(),
		Replicas:     &in.Replicas,
		AppName:      ics.GetAppName(),
		Template:     ics.Spec.Template,
		InstanceName: ics.Name,
	}

	req := &deployflow.DeployNonUpdateRequest{
		Action:            setting.Scale,
		ApplicationSpec:   &applicationSpec,
		NonUpdateStrategy: strategy,
	}
	updated, err := deployflow.CreateNonUpdateDeploy(req, in.Instance.Namespace, cl, logger)
	if err != nil {
		if terrors.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "application not found")
		} else if terrors.IsConflict(err) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.ScaleReply{DeployName: updated.Name}, nil
}

func (s *Service) Rollback(_ context.Context, in *pb.RollbackRequest) (*pb.RollbackReply, error) {
	logger := log.WithFields(logrus.Fields{
		"context":      "application",
		"namespace":    in.Instance.Namespace,
		"instanceName": in.Instance.Name,
	})
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	var strategy *tritonappsv1alpha1.DeployUpdateStrategy = nil
	if in.Strategy != nil {
		size := intstr.Parse(in.Strategy.BatchSize)
		strategy = &tritonappsv1alpha1.DeployUpdateStrategy{
			BaseStrategy: tritonappsv1alpha1.BaseStrategy{
				BatchSize:            &size,
				Batches:              int(in.Strategy.Batches),
				BatchIntervalSeconds: in.Strategy.BatchIntervalSeconds,
				Mode:                 tritonappsv1alpha1.DeployMode(in.Strategy.Mode),
			},
			NoPullIn: in.Strategy.NoPullIn,
			Canary:   int(in.Strategy.Canary),
			Stage:    tritonappsv1alpha1.BatchPhase(in.Strategy.Stage),
		}
	}

	logger.Infof("Start to rollback application %s", in.Instance.Name)

	updated, oldName, err := deployflow.RollbackDeploy(in.Instance.Namespace, in.Instance.Name, in.DeployName, cl, strategy, logger)
	if err != nil {
		if terrors.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "application not found")
		} else if terrors.IsConflict(err) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.RollbackReply{DeployName: updated.Name, RollbackTo: oldName}, nil
}

func setInstanceReply(cs *kruiseappsv1alpha1.CloneSet) *pb.Instance {
	ics := internalcloneset.FromCloneSet(cs)

	return &pb.Instance{
		Name:              ics.GetName(),
		Namespace:         ics.GetNamespace(),
		AvailableReplicas: ics.Status.UpdatedReadyReplicas,
		Replicas:          *ics.Spec.Replicas,
		Status:            ics.GetPhase(),
		AppID:             int32(ics.GetAppID()),
		GroupID:           int32(ics.GetGroupID()),
		Image:             ics.GetAppImage(),
		Cpu:               ics.GetAppCPU(),
		Memory:            ics.GetAppMemory(),
		UpdateRevision:    ics.GetUpdateRevision(),
	}
}

func setInstancesReply(css []*kruiseappsv1alpha1.CloneSet) []*pb.Instance {
	res := make([]*pb.Instance, 0, len(css))
	for _, cs := range css {
		res = append(res, setInstanceReply(cs))
	}

	return res
}

func getInstance(ns, name string) (*kruiseappsv1alpha1.CloneSet, error) {
	cs, found, err := fetcher.GetCloneSetInCache(ns, name, kubeclient.NewManager().GetClient())
	if err != nil {
		log.WithError(err).Error("failed to fetch application")
		return nil, status.Error(codes.Internal, "failed to fetch application")
	} else if !found {
		return nil, status.Error(codes.NotFound, "application not found")
	}

	return cs, nil
}
