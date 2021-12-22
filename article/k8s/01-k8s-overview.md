---
title: 'k8s系列(一)--初识k8s整体架构'
tags: ["k8s"]
categories: ["k8s"]
date: "2021-12-15T20:02:32+08:00"
toc: true
draft: false
---



k8s包括control plane 和 compute nodes组成。

<!--more-->



![image-20211215200441408](/images/k8s/k8s-01-1.png)



* Control plane：包括一些必备的控制节点，主要负责对真实node以及其中Pod的管控。
* Compute nodes：真实计算节点，运行着程序和任务的实例。



## 1. Control Nodes

### 1.1 kube-api-server

它是k8s cluster的核心管理组件。首先它可以接受用户的创建请求，对请求去做一些auth鉴权和validate验证。之后kube-api-server会把这些请求配置(yaml)写入到etcd中去，写入成功后etcd会告诉kube-api-server。然后kube-api-server会回去创建响应的实例，当创建完成的时候会告诉用户请求创建完成。

### 1.2 etcd

etcd是一个基于raft+boltdb实现的key-value数据库，它用于整个集群的配置元信息存储，实例的state状态，wordloads等。

### 1.3 scheduler

Scheduler 调度程序时刻密切关注着nodes的工作负载。在任期中，它会每隔一段换时间对 kube-api-server 执行ping操作，通常是5s，目的是为了确定是否有一些工作负载需要进行调度。

```
scheduler问：是否有一些工作负载需要创建？
api-server答：No ok！

......5s

scheduler问：现在呢？是否有一些工作负载需要创建？
api-server答：All right！
```

一旦有一些node的负载情况不再满足响应的限制规则，比如管理员修改了某些设置导致，磁盘空间不足等。这时scheduler会做出调度决策，选择一个更适合的node环境运行pod。

*scheduler做出选择之后，会立刻进行调度吗？*No，No！！

它不会直接做出调度，它会告诉kube-api-server如何去做，而不是自己做。

*kube-api-server收到后会立刻去做出调度吗？*No！！

它会先将一些配置写入etcd，当写入成功之后，拿到对目标state，kube-api-server会知道如何去做，并通过向目标node的kubelet发送指令，kubelet会与CRI密切合作完成pod的创建。

### 1.4 controller-manager

管理着所有的controller，所有的controller密切关注这整个k8s系统的不同部分。保证目的状态和真实状态的一致性。

* replication controller
* Create pod controller
* Ping controller
* ....



## 2. Compute Nodes

### 2.1 kubelet

kubelet是控制平面（控制节点）与计算节点交流的中间媒介。负责接收来自控制平面的请求，在node中于CRI密切合作创建相应的pod。

在每一个计算节点上都会有一个kubelet，kubelet负责把当前节点注册到k8s集群中去。同时也会定时发送心跳到kube-api-server，以便kube-api-server能够知道哪些node处于健康状态。

当需要做出调度的时候，kube-api-server会向kubelet发出指令，来创建或销毁工作负载pod。

### 2.2 kube-proxy

主要负责node中的流量转发，与其他的node通信都会经过kube-proxy。

### 2.3 CRI

CRI 是指符合容器运行时标准的容器运行时引擎，比如docker，containerd等。





## 3. 基本概念总览



![img](/images/k8s/k8s-01-2.png)



![img](/images/k8s/k8s-01-3.png)



* https://www.bilibili.com/video/BV19F411b7NN
