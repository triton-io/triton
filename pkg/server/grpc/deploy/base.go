package deploy

import (
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	"github.com/triton-io/triton/pkg/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	kubeclient "github.com/triton-io/triton/pkg/kube/client"
	internaldeploy "github.com/triton-io/triton/pkg/kube/types/deploy"
	pb "github.com/triton-io/triton/pkg/protos/deployflow"
)

func setDeployReply(d *tritonappsv1alpha1.DeployFlow) *pb.DeployReply {
	if d == nil {
		return nil
	}
	return &pb.DeployReply{Deploy: setDeploy(d)}
}

func setDeploysReply(ds []*tritonappsv1alpha1.DeployFlow) *pb.DeploysReply {
	rs := make([]*pb.Deploy, 0, len(ds))
	for _, d := range ds {
		rs = append(rs, setDeploy(d))
	}

	return &pb.DeploysReply{Deploys: rs}
}

func setDeploy(d *tritonappsv1alpha1.DeployFlow) *pb.Deploy {
	start, _ := ptypes.TimestampProto(d.Status.StartedAt.Time)
	end, _ := ptypes.TimestampProto(d.Status.FinishedAt.Time)
	updated, _ := ptypes.TimestampProto(d.Status.UpdatedAt.Time)
	idl := internaldeploy.FromDeploy(d)

	cs := make([]*pb.Batch, 0, len(d.Status.Conditions))
	for _, c := range d.Status.Conditions {
		pods := make([]*pb.PodInfo, 0, len(c.Pods))
		for _, p := range c.Pods {
			pods = append(pods, &pb.PodInfo{
				Name:         p.Name,
				Ip:           p.IP,
				Port:         p.Port,
				Phase:        p.Phase,
				PullInStatus: p.PullInStatus,
			})
		}

		start, _ := ptypes.TimestampProto(c.StartedAt.Time)
		end, _ := ptypes.TimestampProto(c.FinishedAt.Time)

		cs = append(cs, &pb.Batch{
			Batch:          int32(c.Batch),
			BatchSize:      int32(c.BatchSize),
			Canary:         c.Canary,
			Phase:          string(c.Phase),
			FailedReplicas: int32(c.FailedReplicas),
			Pods:           pods,
			StartedAt:      start,
			FinishedAt:     end,
		})
	}

	return &pb.Deploy{
		Name:                 d.Name,
		AppID:                int32(d.Spec.Application.AppID),
		GroupID:              int32(d.Spec.Application.GroupID),
		Namespace:            d.Namespace,
		AppName:              d.Spec.Application.AppName,
		InstanceName:         d.Spec.Application.InstanceName,
		Replicas:             *d.Spec.Application.Replicas,
		Action:               d.Spec.Action,
		Mode:                 string(idl.Mode()),
		BatchIntervalSeconds: idl.BatchIntervalSeconds(),
		Canary:               int32(idl.Canary()),

		AvailableReplicas:    d.Status.AvailableReplicas,
		UpdatedReplicas:      d.Status.UpdatedReplicas,
		UpdatedReadyReplicas: d.Status.UpdatedReadyReplicas,
		UpdateRevision:       idl.GetUpdateRevision(),
		Conditions:           cs,
		Paused:               d.Status.Paused,
		Phase:                string(d.Status.Phase),
		Finished:             d.Status.Finished,
		Batches:              int32(d.Status.Batches),
		BatchSize:            idl.BatchSizeNum(),
		FinishedBatches:      int32(d.Status.FinishedBatches),
		FinishedReplicas:     int32(d.Status.FinishedReplicas),
		FailedReplicas:       int32(d.Status.FailedReplicas),
		StartedAt:            start,
		FinishedAt:           end,
		UpdatedAt:            updated,
	}
}

func getDeploy(ns, name string) (*tritonappsv1alpha1.DeployFlow, error) {
	d, found, err := fetcher.GetDeployInCache(ns, name, kubeclient.NewManager().GetClient())
	if err != nil {
		log.WithError(err).Error("failed to get deploy")
		return nil, status.Error(codes.Internal, "failed to get deploy")
	} else if !found {
		return nil, status.Error(codes.NotFound, "deploy not found")
	}

	return d, nil
}

func getDeploys(f fetcher.DeployFilter) ([]*tritonappsv1alpha1.DeployFlow, error) {
	ds, err := fetcher.GetDeploysInCache(f, kubeclient.NewManager().GetClient())
	if err != nil {
		log.WithError(err).Error("failed to get deploys")
		return nil, status.Error(codes.Internal, "failed to get deploys")
	}

	return ds, nil
}

func grpcFilterToFetcherFilter(f *pb.DeployFilter) fetcher.DeployFilter {
	var after *time.Time = nil
	if f.After != nil {
		t, err := ptypes.Timestamp(f.After)
		if err == nil {
			after = &t
		}
	}

	return fetcher.DeployFilter{
		Namespace:    f.Namespace,
		InstanceName: f.InstanceName,
		Start:        int(f.Start),
		PageSize:     int(f.PageSize),
		After:        after,
	}
}

func matchFilter(f fetcher.DeployFilter, d *tritonappsv1alpha1.DeployFlow) bool {
	if f.Namespace != "" && d.Namespace != f.Namespace {
		return false
	}

	if f.InstanceName != "" && d.Spec.Application.InstanceName != f.InstanceName {
		return false
	}

	updatedTime := d.Status.UpdatedAt
	if updatedTime.IsZero() {
		updatedTime = d.CreationTimestamp
	}
	if f.After != nil && updatedTime.Time.Before(*f.After) {
		return false
	}

	return true
}
