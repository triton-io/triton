package deploy

import (
	"context"

	pb "github.com/triton-io/triton/pkg/protos/deployflow"
)

type StreamSender interface {
	Send(*pb.DeployReply) error
	Context() context.Context
}
