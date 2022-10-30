---
title: 'k8s系列(三)--实操-使用minikube部署redis'
tags: ["k8s", "Redis"]
categories: ["k8s"]
date: "2022-10-30T13:40:37+08:00"
toc: true
draft: false
---

k8s系列之应用部署实操---使用minikube部署redis，内容包括单节点实例部署redis，集群化部署redis的集群等。

<!--more-->

## 单节点部署

### 1. 首先创建namespaces

```shell
kubectl create namespace redis-ns
```

### 2. 编写redis-config.yml，使用命令创建configMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: redis-config
  namespace: redis-ns
  labels:
    app: redis
data:
  redis.conf: |-
    dir /srv
    port 6379
    bind 0.0.0.0
    appendonly yes
    daemonize no
    #protected-mode no
    requirepass test
    pidfile /srv/redis-6379.pid
```

使用命令创建configMap

```bash
➜  kubectl apply -f redis-config.yaml
configmap/redis-config created
```

### 3. 编写redis的Deployment、Service配置，并使用命令创建

```yml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: redis-ns
  labels:
    app: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:5.0.7
        command:
          - "sh"
          - "-c"
          - "redis-server /usr/local/redis/redis.conf"
        ports:
        - containerPort: 6379
        resources:
          limits:
            cpu: 1000m
            memory: 1024Mi
          requests:
            cpu: 1000m
            memory: 1024Mi
        livenessProbe:
          tcpSocket:
            port: 6379
          initialDelaySeconds: 300
          timeoutSeconds: 1
          periodSeconds: 10
          successThreshold: 1
          failureThreshold: 3
        readinessProbe:
          tcpSocket:
            port: 6379
          initialDelaySeconds: 5
          timeoutSeconds: 1
          periodSeconds: 10
          successThreshold: 1
          failureThreshold: 3
        volumeMounts:
        - name: config
          mountPath:  /usr/local/redis/redis.conf
          subPath: redis.conf
      volumes:
      - name: config
        configMap:
          name: redis-config
---
apiVersion: v1
kind: Service
metadata:
  name: service-redis
  namespace: redis-ns
spec:
  ports:
    - port: 6379
      protocol: TCP
      targetPort: 6379
      nodePort: 30120
  selector:
    app: redis
  type: NodePort
```

使用命令创建deployment和service

```shell
➜  kubectl apply -f redis.yaml
deployment.apps/redis created
service/service-redis created
```

查看资源

```shell
# kubectl get service,deploy,pod -n redis-ns -o wide
➜  kubectl get all -n redis-ns -o wide
NAME                         READY   STATUS    RESTARTS   AGE     IP           NODE       NOMINATED NODE   READINESS GATES
pod/redis-66fd8f7cd7-4qg5k   1/1     Running   0          7m14s   172.17.0.4   minikube   <none>           <none>

NAME                    TYPE       CLUSTER-IP       EXTERNAL-IP   PORT(S)          AGE     SELECTOR
service/service-redis   NodePort   10.110.231.204   <none>        6379:30120/TCP   7m14s   app=redis

NAME                    READY   UP-TO-DATE   AVAILABLE   AGE     CONTAINERS   IMAGES        SELECTOR
deployment.apps/redis   1/1     1            1           7m14s   redis        redis:5.0.7   app=redis

NAME                               DESIRED   CURRENT   READY   AGE     CONTAINERS   IMAGES        SELECTOR
replicaset.apps/redis-66fd8f7cd7   1         1         1       7m14s   redis        redis:5.0.7   app=redis,pod-template-hash=66fd8f7cd7
➜  
```

### 4. 使用redis-cli连接redis验证

**方式一**：直接进入容器内部使用redis-cli验证，密码的话在第一步创建的ConfigMap中

```shell
➜  kubectl -n redis-ns exec -it redis-66fd8f7cd7-4qg5k --sh
# redis-cli
127.0.0.1:6379> auth test
OK
127.0.0.1:6379> config get requirepass
1) "requirepass"
2) "test"
127.0.0.1:6379>
```

**方式二**：在主机操作系统上，通过redis-cli程序连接k8s集群中的pod的ip端口进行redis访问。

下载源码编译安装redis-cli：

```shell
wget http://download.redis.io/redis-stable.tar.gz

tar -zxvf redis-stable.tar.gz

cd redis-stable/

make redis-cli
```

我目前使用minikube+docker安装部署，需要进行如下一些特殊操作。下面的命令会启动一个单独的进程运行，创建一条到集群的隧道。该命令将服务直接公开给主机操作系统上运行的任何程序。通过这条隧道，我们在主机操作系统就可直接使用redis-cli连接集群的redis实例了。

具体原因与解释：

> **NodePort access**
>
> A NodePort service is the most basic way to get external traffic directly to your service. NodePort, as the name implies, opens a specific port, and any traffic that is sent to this port is forwarded to the service.
>
> **Getting the NodePort using the service command**
>
> We also have a shortcut for fetching the minikube IP and a service’s `NodePort`:
>
> ```shell
> minikube service <service-name> --url
> ```
>
> **Using `minikube service` with tunnel**
>
> The network is limited if using the Docker driver on Darwin, Windows, or WSL, and the Node IP is not reachable directly.
>
> Running minikube on Linux with the Docker driver will result in no tunnel being created.
>
> Services of type `NodePort` can be exposed via the `minikube service <service-name> --url` command. It must be run in a separate terminal window to keep the [tunnel](https://en.wikipedia.org/wiki/Port_forwarding#Local_port_forwarding) open. Ctrl-C in the terminal can be used to terminate the process at which time the network routes will be cleaned up.

具体操作：

```shell
minikube service service-redis --url
```

新开一个终端上查看ssh隧道信息

```shell
➜  ps -ef | grep docker@127.0.0.1
  502 84690 83944   0  8:47下午 ttys003    0:00.00 grep --color=auto --exclude-dir=.bzr --exclude-dir=CVS --exclude-dir=.git --exclude-dir=.hg --exclude-dir=.svn --exclude-dir=.idea --exclude-dir=.tox docker@127.0.0.1
  502 84311 84286   0  8:34下午 ttys007    0:00.02 ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -N docker@127.0.0.1 -p 61631 -i /Users/devhg/.minikube/machines/minikube/id_rsa -L 62451:10.110.231.204:6379
```

> $ ps -ef | grep docker@127.0.0.1
> ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -N docker@127.0.0.1 -p 55972 -i /Users/FOO/.minikube/machines/minikube/id_rsa -L TUNNEL_PORT:CLUSTER_IP:TARGET_PORT
>
> TUNNEL_PORT：隧道端口
>
> CLUSTER_IP：service入口ip
>
> TARGET_PORT：实例端口

使用TUNNEL_PORT连接k8s集群的redis实例，`redis-cli -h 127.0.0.1 -p $TUNNEL_PORT`

```shell
➜  redis-stable ./src/redis-cli -h 127.0.0.1 -p 62451
127.0.0.1:62451> keys *
(error) NOAUTH Authentication required.
127.0.0.1:62451> auth test
OK
127.0.0.1:62451> keys *
(empty array)
127.0.0.1:62451> exit
```

参考资料

* https://github.com/kubernetes/minikube/issues/13747
* https://developer.aliyun.com/article/933008
* https://minikube.sigs.k8s.io/docs/handbook/accessing/

## 集群化部署(WIP)
