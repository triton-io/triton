# Overview

Kubernetes provides a set of default controllers for workload management,
like StatefulSet, Deployment, DaemonSet for instances. While at the same time, kubernetes does not provide a way to deploy these resource 
which has a rich and secure strategy, such as canary strategy, blue/green strategy.

As we all know, whether it is a stateless Deployment or a stateful StatefulSet, Kubernetes supports rolling updates of services. 
You often see in some articles about rolling release and grayscale release of applications based on native Deployment. 
This method of development is simple and easy to use, but it brings many defects and cannot control the release process, such as not supports the ability to pause and continue during the release process, 
and to pull in and out the traffic gracefully.

In order to achieve a richer release strategy and more fine-grained release control to ensure the safety and stability of the container release process, 
Triton chose OpenKruise as the application workload. OpenKruise is a standard extension of Kubernetes. It can be used with native Kubernetes and provides more powerful and efficient capabilities for managing application containers, sidecars, and image distribution.

OpenKruise provides many enhanced resources of native Kubernetes, such as CloneSet, Advanced StatefulSet, SidecarSet, etc. 
Among them, CloneSet provides more efficient, deterministic and controllable application management and deployment capabilities, supports elegant in-place upgrades, designated deletions, configurable release order, and parallel/gradual releases, which can meet more diverse applications. 
 So Triton chose to implement the release process of stateless applications based on CloneSet.

## Benefits
- Triton provides a cloud-native way to deploy applications, which is safe, controllable, and policy-rich.

- Base on OpenKruise to serve new workloads, Kruise offers extensions to default
  controllers for new capabilities.  Ideally, it can be the first option to consider when one wants to extend upstream Kubernetes for workload management.
  
- Triton plans to offer more Kubernetes automation solutions in the
  areas of scaling, QoS and traffic operators, etc. Stay tuned!

## Tutorials

Several [Tutorials](./tutorial/README.md) are provided to demonstrate how to use the controllers
