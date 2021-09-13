# Triton-io/Triton

[![License](https://img.shields.io/badge/license-Apache%202-4EB1BA.svg)](https://www.apache.org/licenses/LICENSE-2.0.html)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/2908/badge)](https://bestpractices.coreinfrastructure.org/en/projects/2908)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](./CODE_OF_CONDUCT.md)

English | [简体中文](./README-zh_CN.md)

## Introduction

Triton provides a cloud-native DeployFlow, which is safe, controllable, and policy-rich.

For more introduction details, see [Overview docs](./docs/README.md)

## Key Features

- **Canary Deploy**

   Triton support `Canary` deploy strategy and provide a k8s service-based traffic pull-in and pull-out solution. 

- **Deploy in batches**

    Triton can deploy your application in several batches and you can pause/resume in deploy process.

- **REST && GRPC API**

   Triton provides many APIs to make deploy easy, such as `Next`, `Cancel`,`Pause`,`Resume`,`Scale`, `Gets`,`Restart` etc. 

- **Selective pods to scale/restart**

  User can scale selective pods or restart these pods.

- **Base on OpenKruise**

  Triton use [OpenKruise](https://openkruise.io/en-us/docs/what_is_openkruise.html) as workloads which have more powerful capabilities

- **...**

## Quick Start

For a Kubernetes cluster with its version higher than v1.13, you can simply install Triton with helm v3.1.0+:
// TODO

Note that installing this chart directly means it will use the default template values for the triton-manager.

For more details, see [installation doc](./docs/installation/README.md).

## Documentation

We provide [**tutorials**](./docs/tutorial/README.md) for Triton controllers to demonstrate how to use them.

We also provide [**debug guide**](./docs/debug/README.md) for Triton developers to help to debug. 

## Users


## Contributing

You are warmly welcome to hack on Triton. We have prepared a detailed guide [CONTRIBUTING.md](CONTRIBUTING.md).

## Community

Active communication channels:

- WeChat Group: 

<div>
  <img src="docs/img/triton-dev-group.JPG" width="280" title="wechat">
</div>


## RoadMap
* [ ] Support custom traffic pull-in
* [ ] Provide helm to install DeployFlow
* [ ] REST & GRPC API doc
* [ ] .......

## License

Triton is licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE.md) for the full license text.


