package pod

import (
	"context"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	"github.com/triton-io/triton/pkg/log"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"

	kubeclient "github.com/triton-io/triton/pkg/kube/client"
	internalpod "github.com/triton-io/triton/pkg/kube/types/pod"
	pb "github.com/triton-io/triton/pkg/protos/pod"
	podservice "github.com/triton-io/triton/pkg/services/pod"
)

type Service struct {
	pb.UnimplementedPodServer
}

func (s *Service) Gets(_ context.Context, in *pb.GetsRequest) (*pb.PodsReply, error) {
	f := &podservice.Filter{
		Namespace:    in.Filter.Namespace,
		InstanceName: in.Filter.InstanceName,
		IP:           in.Filter.Ip,
	}
	pods, err := podservice.FetchPods(f, labels.Everything(), in.NoSort)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.PodsReply{Pods: setPodsReply(pods)}, nil
}

func (s *Service) Get(_ context.Context, in *pb.PodMetaRequest) (*pb.PodReply, error) {
	p, err := getPod(in.Pod.Namespace, in.Pod.Name)
	if err != nil {
		return nil, err
	}

	return &pb.PodReply{Pod: setPodReply(p)}, nil
}

func setPodReply(pod *corev1.Pod) *pb.PodInfo {
	ct, _ := ptypes.TimestampProto(pod.GetCreationTimestamp().Time)
	p := internalpod.FromPod(pod)

	return &pb.PodInfo{
		Name:              p.GetName(),
		Namespace:         p.GetNamespace(),
		CreationTimestamp: ct,
		InstanceName:      p.GetInstanceName(),
		Image:             p.GetAppImage(),
		Cpu:               p.GetAppCPU(),
		Memory:            p.GetAppMemory(),
		GuaranteedCPU:     p.GetAppRequestCPU(),
		GuaranteedMemory:  p.GetAppRequestMemory(),
		ContainerName:     p.GetAppContainerName(),
		PodIP:             p.GetPodIP(),
		Status:            string(p.GetPhase()),
		HostIP:            p.GetHostIP(),
		UpdateRevision:    p.GetUpdateRevision(),
	}
}

func setPodsReply(pods []*corev1.Pod) []*pb.PodInfo {
	res := make([]*pb.PodInfo, 0, len(pods))
	for _, pod := range pods {
		res = append(res, setPodReply(pod))
	}

	return res
}

func getPod(ns, name string) (*corev1.Pod, error) {
	p, found, err := fetcher.GetPodInCache(ns, name, kubeclient.NewManager().GetClient())
	if err != nil {
		log.WithError(err).Error("failed to fetch pod")
		return nil, status.Error(codes.Internal, "failed to fetch pod")
	} else if !found {
		return nil, status.Error(codes.NotFound, "pod not found")
	}

	return p, nil
}
