# Triton

## Triton 概述

伴随着云原生技术在越来越多的企业落地，如今的 Kubernetes 和容器已经完全进入主流市场，成为云计算的新界面，帮助企业充分享受云原生的优势，加速应用迭代创新效率，降低开发运维成本。但在向着云原生架构转型的过程中，也有许多问题需要被解决。比如原生 Kubernetes 的复杂性、容器化应用的生命周期管理，以及向以容器为基础设施迁移的过程中可能出现的服务稳定性挑战等等。

开源云原生应用发布组件 Triton 的出现，就是为了解决企业应用在容器化过程中安全落地生产的问题。Triton 以 [OpenKruise](https://openkruise.io/zh-cn/docs/what_is_openkruise.html "OpenKruise") 作为容器应用自动化引擎，实现应用负载的扩展增强能力，为原有持续交付系统带来全面升级，不仅解决了应用生命周期管理的问题，包括开发、部署、运维等，同时打通微服务治理，可以帮助研发提升持续交付的效率。

有关 Trion 设计方案、实现原理的详细介绍可以参考[这篇文章](https://mp.weixin.qq.com/s/CAFNxnKkS5PjeE8ywfoQqQ)。本文将从源码级安装、debug、demo 应用发布演示三个方面介绍 Triton 的核心特性以及 Triton 的快速上手使用、开发，最后介绍 Triton 的 Roadmap。由于时间关系，一键安装、Helm 安装的方式正在开发中，会在正式版本 release 中提供。

## 核心能力

本次带来的 v0.1.0 开源版本在代码上进行了重构，暂时去掉了对网络方案、微服务架构的依赖，抽象出应用模型的概念，更具备普适性，核心特性如下：

- 全托管于 k8s 集群，便于组件安装、维护和升级；

- 支持使用 API 和 kubectl 插件（规划中）完成应用创建、部署、升级，并支持单批发布、分批发布和金丝雀发布；

- 提供从创建到运行的应用全生命周期管理服务，包括应用的发布、启动、停止、扩容、缩容和删除等服务，可以轻松管理上千个应用实例的交付问题；

- Triton 提供了大量 API 来简化部署等操作，如Next、Cancel、Pause、Resume、Scale、Gets、Restart 等，轻松对接公司内部的 PaaS 系统；

  

  ## 操作指南

  在开始之前，检查一下当前环境是否满足一下前提条件:

  1. 确保环境能够与 kube-apiserver 连通；

  2. 确保 `OpenKruise` 已经在当前操作 k8s 集群安装，若未安装可以参考[文档](https://openkruise.io/en-us/docs/installation.html "OpenKruise 安装文档")；

  3. 确保有 Golang 开发环境，Fork & git clone 代码后，执行 `make install` 安装 CRD `DeployFlow` ；

  4. 操作 API 的过程中需要 `grpcurl` 这个工具，参考 [grpcurl 文档](https://github.com/fullstorydev/grpcurl "grpcurl 安装文档")进行安装；

  

  ### 创建 DeployFlow 来发布 Nginx Demo Application

  #### 运行 DeployFlow controller

  进入到代码根目录下执行  `make run`

  #### 创建 DeployFlow 准备发布应用

  ```bash
  kubectl apply -f https://github.com/triton-io/triton/raw/main/docs/tutorial/v1/nginx-deployflow.yaml
  ```

  ![](https://tva1.sinaimg.cn/large/008i3skNly1guf6qrw2zpj61tq04cmya02.jpg)

  

  会创建出一个 DeployFlow 资源和本应用对应的 Service，可以查看该 yaml 文件了解详细的 DeployFlow 定义。

  ```yaml
  apiVersion: apps.triton.io/v1alpha1
  kind: DeployFlow
  metadata:
    labels:
      app: "12122"
      app.kubernetes.io/instance: 12122-sample-10010
      app.kubernetes.io/name: deploy-demo-hello
      group: "10010"
      managed-by: triton-io
    name: 12122-sample-10010-df
    namespace: default
  spec:
    action: create
    application:
      appID: 12122
      appName: deploy-demo-hello
      groupID: 10010
      instanceName: 12122-sample-10010
      replicas: 3
      selector:
        matchLabels:
          app: "12122"
          app.kubernetes.io/instance: 12122-sample-10010
          app.kubernetes.io/name: deploy-demo-hello
          group: "10010"
          managed-by: triton-io
      template:
        metadata: {}
        spec:
          containers:
            - image: nginx:latest
              name: 12122-sample-10010-container
              ports:
                - containerPort: 80
                  protocol: TCP
              resources: {}
    updateStrategy:
      batchSize: 1
      batchIntervalSeconds: 10
      canary: 1 # the number of canary batch
      mode: auto # the mode is auto after canary batch
  ```

  可以看到我们本次发布的应用名字是 `12122-sample-10010`，副本数量是 3，批次大小是 1，有一个金丝雀批次，批次大小是 1，发布的模式是 auto，意味着本次发布只会在金丝雀批次和普通批次之间暂停，后续两个批次会以 `batchIntervalSeconds` 为时间间隔自动触发。

  #### 检查 DeployFlow 状态

  ![](https://tva1.sinaimg.cn/large/008i3skNly1guf6slwhwvj62bm04aq3x02.jpg)

  可以看到我们创建出一个名为 `12122-sample-10010-df` 的 DeployFlow 资源，通过展示的字段了解到本次发布分为 3 个批次，当前批次的大小是 1，已升级和已完成的副本数量都是 0。

  启动几十秒后，检查 DeployFlow 的 status 字段：

  ```bash
  kubectl get df  12122-sample-10010-df -o yaml
  ```

  ```yaml
  status:
    availableReplicas: 0
    batches: 3
    conditions:
    - batch: 1
      batchSize: 1
      canary: true
      failedReplicas: 0
      finishedAt: null
      phase: Smoked
      pods:
      - ip: 172.31.230.23
        name: 12122-sample-10010-2mwkt
        phase: ContainersReady
        port: 80
        pullInStatus: ""
      pulledInAt: null
      startedAt: "2021-09-13T12:49:04Z"
    failedReplicas: 0
    finished: false
    finishedAt: null
    finishedBatches: 0
    finishedReplicas: 0
    paused: false
    phase: BatchStarted
    pods:
    - 12122-sample-10010-2mwkt
    replicas: 1
    replicasToProcess: 3
    startedAt: "2021-09-13T12:49:04Z"
    updateRevision: 12122-sample-10010-6ddf9b7cf4
    updatedAt: "2021-09-13T12:49:21Z"
    updatedReadyReplicas: 0
    updatedReplicas: 1
  ```

  可以看到目前在启动的是 `canary` 批次，该批次已经处于 `smoked` 阶段，该批次中的 pod 是 `12122-sample-10010-2mwkt` ，同时也能看到当前批次中 pod 的拉入状态、拉入时间等信息。

  #### 将应用拉入流量

在此之前我们可以先检查一下 Service 的状态：

```bash
kubectl describe svc sample-12122-svc -o yaml
```

从显示的结果来看，pod `12122-sample-10010-2mwkt` 并没有出现在 Service 的 `Endpoints` 中，意味着当前应用没有正式接入流量：

```yaml
Name:              sample-12122-svc
Namespace:         default
Labels:            app=12122
                   app.kubernetes.io/instance=12122-sample-10010
                   app.kubernetes.io/name=deploy-demo-hello
                   group=10010
                   managed-by=triton-io
Annotations:       <none>
Selector:          app.kubernetes.io/instance=12122-sample-10010,app.kubernetes.io/name=deploy-demo-hello,app=12122,group=10010,managed-by=triton-io
Type:              ClusterIP
IP Families:       <none>
IP:                10.22.6.154
IPs:               <none>
Port:              web  80/TCP
TargetPort:        80/TCP
Endpoints:
Session Affinity:  None
Events:            <none>
```

接下来我们执行拉入操作(Bake)，对应 pod 的状态会从 `ContainerReady` 变为 `Ready`，从而被挂载到对应 Service 的 Endpoints 上开始正式接入流量：

```bash
grpcurl --plaintext -d '{"deploy":{"name":"12122-sample-10010-df","namespace":"default"}}' localhost:8099 deployflow.DeployFlow/Next
```

再次检查 DeployFlow，Service，CloneSet 的状态后，发现 Pod 已被挂载到 Endpoints，DeployFlow 的 `UPDATED_READY_REPLICAS` 字段变为 1，金丝雀批次进入 `baking` 阶段，如果此时应用正常工作，我们再次执行上面的 `Next` 操作，将 DeployFlow 置为 `baked` 阶段，表示本批次点火成功，应用流量正常。

#### Rollout 操作

金丝雀批次到达 `baked` 阶段后，执行 `Next` 操作就会进入后面的普通批次发布，由于我们应用的副本数量设置为 3，去掉金丝雀批次中的一个副本后，还剩 2 个，而 batchSize 的大小为 1，所有剩余的普通批次会分两个批次发布，两个批次之间会间隔 10s 触发。

```bash
grpcurl --plaintext -d '{"deploy":{"name":"12122-sample-10010-df","namespace":"default"}}' localhost:8099 deployflow.DeployFlow/Next
```

![](https://tva1.sinaimg.cn/large/008i3skNgy1gufd0ojjm2j62eg04a3z902.jpg)

![](https://tva1.sinaimg.cn/large/008i3skNly1gufb22h1dej62dg04iwfk02.jpg)

最后应用发布完成，检查 DeployFlow 的状态为 `Success`:

![](https://tva1.sinaimg.cn/large/008i3skNgy1gufb2saqe6j629604gq3x02.jpg)

再次查看 Service 的 Endpoints 可以看到本次发布的 3 个副本都已经挂载上去。

再次回顾整个发布流程，可以总结为下面的状态流转图：

![deployflow-status](https://tva1.sinaimg.cn/large/008i3skNly1gufb5ht6w9j61du0ingnw02.jpg)

#### 暂停/继续 DeployFlow

在部署过程中，如果要暂停 DeployFlow，可以执行 `Pause` 操作：

```bash
grpcurl --plaintext -d '{"deploy":{"name":"12122-sample-10010-df","namespace":"default"}}' localhost:8099 deployflow.DeployFlow/Pause

```

可以继续发布了，就执行 `Resume` 操作：

```bash
grpcurl --plaintext -d '{"deploy":{"name":"12122-sample-10010-df","namespace":"default"}}' localhost:8099 deployflow.DeployFlow/Resume

```

#### 取消本次发布

如果在发布过程中，遇到启动失败，或者拉入失败的情况，要取消本次发布，可执行 `Cancel` 操作：

```bash
grpcurl --plaintext -d '{"deploy":{"name":"12122-sample-10010-df","namespace":"default"}}' localhost:8099 deployflow.DeployFlow/Cancel

```

#### 启动一个扩缩容 DeployFlow

同样可以使用 `auto` 或 `mannual` 模式划分多个批次来执行扩缩容操作。 当一个 CloneSet 缩容时，有时用户倾向于删除特定的 Pod，可以使用 podsToDelete 字段实现指定 Pod 缩容：

```bash
kubectl get pod | grep 12122-sample
12122-sample-10010-2mwkt                      1/1     Running             0          29m
12122-sample-10010-hgdp6                      1/1     Running             0          9m55s
12122-sample-10010-zh98f                      1/1     Running             0          10m
```

我们在缩容的时候指定被缩掉的 Pod 为 `12122-sample-10010-zh98f  `:

```bash
grpcurl --plaintext -d '{"instance":{"name":"12122-sample-10010","namespace":"default"},"replicas":2,"strategy":{"podsToDelete":["12122-sample-10010-zh98f"],"batchSize":"1","batches":"1","batchIntervalSeconds":10}}' \
localhost:8099 application.Application/Scale
{
  "deployName": "12122-sample-10010-kvn6b"
}

❯ kubectl get pod | grep 12122-sample
12122-sample-10010-2mwkt                      1/1     Running             0          29m
12122-sample-10010-zh98f                      1/1     Running             0          11m


```

CloneSet 被缩容为 2 个副本，被缩容的 Pod 正是指定的那个。该功能的实现得益于 OpenKruise 中增强型无状态 workload CloneSet 提供的能力，具体的功能描述可以参考 OpenKruise 文档。

在操作过程中，Triton 也提供了 `Get` 方法实时获取当前 DeployFlow 的 Pod 信息：

```bash
grpcurl --plaintext -d '{"deploy":{"name":"12122-sample-10010-df","namespace":"default"}}' localhost:8099 deployflow.DeployFlow/Get
```

```json
{
  "deploy": {
    "namespace": "default",
    "name": "12122-sample-10010-df",
    "appID": 12122,
    "groupID": 10010,
    "appName": "deploy-demo-hello",
    "instanceName": "12122-sample-10010",
    "replicas": 3,
    "action": "create",
    "availableReplicas": 3,
    "updatedReplicas": 3,
    "updatedReadyReplicas": 3,
    "updateRevision": "6ddf9b7cf4",
    "conditions": [
      {
        "batch": 1,
        "batchSize": 1,
        "canary": true,
        "phase": "Baked",
        "pods": [
          {
            "name": "12122-sample-10010-2mwkt",
            "ip": "172.31.230.23",
            "port": 80,
            "phase": "Ready",
            "pullInStatus": "PullInSucceeded"
          }
        ],
        "startedAt": "2021-09-13T12:49:04Z",
        "finishedAt": "2021-09-13T13:07:43Z"
      },
      {
        "batch": 2,
        "batchSize": 1,
        "phase": "Baked",
        "pods": [
          {
            "name": "12122-sample-10010-zh98f",
            "ip": "172.31.226.94",
            "port": 80,
            "phase": "Ready",
            "pullInStatus": "PullInSucceeded"
          }
        ],
        "startedAt": "2021-09-13T13:07:46Z",
        "finishedAt": "2021-09-13T13:08:03Z"
      },
      {
        "batch": 3,
        "batchSize": 1,
        "phase": "Baked",
        "pods": [
          {
            "name": "12122-sample-10010-hgdp6",
            "ip": "172.31.227.215",
            "port": 80,
            "phase": "Ready",
            "pullInStatus": "PullInSucceeded"
          }
        ],
        "startedAt": "2021-09-13T13:08:15Z",
        "finishedAt": "2021-09-13T13:08:45Z"
      }
    ],
    "phase": "Success",
    "finished": true,
    "batches": 3,
    "batchSize": 1,
    "finishedBatches": 3,
    "finishedReplicas": 3,
    "startedAt": "2021-09-13T12:49:04Z",
    "finishedAt": "2021-09-13T13:08:45Z",
    "mode": "auto",
    "batchIntervalSeconds": 10,
    "canary": 1,
    "updatedAt": "2021-09-13T13:08:45Z"
  }
}
```

## TODOS

上面演示的就是 Triton 提供的核心能力。对于基础团队来说，Triton 不仅仅是一个开源项目，它也是一个真实的比较接地气的云原生持续交付项目。通过开源，我们希望 Triton 能丰富云原生社区的持续交付工具体系，为更多开发者和企业搭建云原生化的 PaaS 平台助力，提供一种现代的、高效的的技术方案。

开源只是迈出的一小步，未来我们会进一步推动 Triton 不断走向完善，包括但不限于以下几点：

- 支持自定义注册中心，可以看到目前 Triton 采用的是 k8s 原生的 Service 作为应用的注册中心，但据我们所了解，很多企业都使用自定义的注册中心，比如 spring cloud 的 Nacos 等；
- 提供 helm 安装方式；
- 完善 REST & GRPC API 以及相应文档；
- 结合内外部用户需求，持续迭代。项目开源后，我们也会根据开发者需求开展迭代。

欢迎大家向 Triton 提交 issue 和 PR 共建 Triton 社区。我们诚心期待更多的开发者加入，也期待 Triton 能够助力越来越多的企业快速构建云原生持续交付平台。如果有企业或者用户感兴趣，我们可以提供专项技术支持和交流，欢迎入群咨询。

### 相关链接

项目地址：https://github.com/triton-io/triton

### 交流群

<img src="https://tva1.sinaimg.cn/large/008i3skNgy1gufd1pbwh9j60e60iuabg02.jpg" alt="triton-wechat" style="zoom:67%;" />



